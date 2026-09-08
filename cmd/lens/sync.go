package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	swift "github.com/ncw/swift/v2"
	"github.com/phuslu/log"
)

// validateSwiftConfig checks that the required Swift settings and one complete auth method
// are present before any connection is attempted.
func validateSwiftConfig() error {
	var missing []string
	if strings.TrimSpace(SwiftAuthURL) == "" {
		missing = append(missing, "SWIFT_AUTHURL")
	}
	if strings.TrimSpace(SwiftContainer) == "" {
		missing = append(missing, "SWIFT_CONTAINER")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing Swift configuration: %s", strings.Join(missing, ", "))
	}

	if !UseAppCredential() && !usePasswordAuth() {
		return errors.New("swift auth not configured: set either " +
			"SWIFT_APPLICATION_CREDENTIAL_ID + SWIFT_APPLICATION_CREDENTIAL_SECRET " +
			"(recommended for federated/SSO clouds), or " +
			"SWIFT_USERNAME + SWIFT_APIKEY + SWIFT_DOMAIN + SWIFT_TENANT")
	}
	return nil
}

// errorBodyLogger logs the response body of failed Swift requests. The swift client maps
// a status code to a bare error such as "Too Large Object" and discards the body, but Swift
// puts the actual reason there (e.g. "Upload exceeds quota.").
type errorBodyLogger struct {
	next http.RoundTripper
}

const maxLoggedErrorBody = 2048

func (t *errorBodyLogger) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.next.RoundTrip(req)
	if err != nil || resp.StatusCode < 400 {
		return resp, err
	}

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxLoggedErrorBody))
	if readErr != nil {
		return resp, nil
	}
	// Put the consumed bytes back so the client still sees a complete body.
	resp.Body = struct {
		io.Reader
		io.Closer
	}{io.MultiReader(bytes.NewReader(body), resp.Body), resp.Body}

	log.Warn().Msgf("Swift %s %s -> %s: %s", req.Method, req.URL.Path, resp.Status, strings.TrimSpace(string(body)))
	return resp, nil
}

// newSwiftTransport mirrors the defaults the swift client would install itself, wrapped so
// that failed-request bodies get logged.
func newSwiftTransport() http.RoundTripper {
	return &errorBodyLogger{next: &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		// Half of linux's default open files limit (1024).
		MaxIdleConnsPerHost:   512,
		ExpectContinueTimeout: 5 * time.Second,
	}}
}

// isRetryable reports whether err is worth another attempt. Client errors (4xx) other than
// timeout / rate limiting are deterministic — retrying a 413 or a 403 only wastes time.
func isRetryable(err error) bool {
	var swiftErr *swift.Error
	if !errors.As(err, &swiftErr) {
		return true // network / transport error
	}
	code := swiftErr.StatusCode
	switch code {
	case http.StatusRequestTimeout, http.StatusTooManyRequests, 498: // 498 = Swift rate limited
		return true
	}
	return code < 400 || code >= 500
}

// newSwiftConnection builds a Keystone v3 connection. Application-credential auth is preferred
// when configured — it is the headless-friendly method on federated/SSO clouds, needing no
// username, domain, or tenant. Otherwise it falls back to username/password, mirroring the
// python-swiftclient setup (Domain = user domain, TenantDomain = project domain,
// Tenant = project name).
func newSwiftConnection() *swift.Connection {
	if UseAppCredential() {
		return &swift.Connection{
			ApplicationCredentialId:     SwiftAppCredID,
			ApplicationCredentialSecret: SwiftAppCredSecret,
			AuthUrl:                     SwiftAuthURL,
			Region:                      SwiftRegion,
			AuthVersion:                 3,
			Transport:                   newSwiftTransport(),
		}
	}
	return &swift.Connection{
		UserName:     SwiftUsername,
		ApiKey:       SwiftAPIKey,
		AuthUrl:      SwiftAuthURL,
		Domain:       SwiftDomain, // user_domain_name
		Tenant:       SwiftTenant, // project (tenant) name
		TenantDomain: SwiftDomain, // project_domain_name
		Region:       SwiftRegion,
		AuthVersion:  3,
		Transport:    newSwiftTransport(),
	}
}

// NewSwiftConn returns an authenticated Keystone v3 Swift connection built from the
// process configuration.
func NewSwiftConn() (*swift.Connection, error) {
	if err := validateSwiftConfig(); err != nil {
		return nil, err
	}

	conn := newSwiftConnection()
	if err := conn.Authenticate(context.Background()); err != nil {
		return nil, fmt.Errorf("swift auth: %w", err)
	}
	return conn, nil
}

func TestSwiftConnection() error {
	conn, err := NewSwiftConn()
	if err != nil {
		return err
	}

	containers, err := conn.ContainerNames(context.Background(), nil)
	if err != nil {
		return fmt.Errorf("failed to list Swift containers: %w", err)
	}

	log.Info().Msg("Swift containers:")
	for _, container := range containers {
		log.Info().Msgf(" - %s", container)
	}
	return nil
}

// withRetry runs fn up to attempts times with exponential backoff, returning the last error.
// It gives up immediately on errors that cannot succeed on a retry.
func withRetry(attempts int, fn func() error) error {
	var err error
	backoff := time.Second
	for i := range attempts {
		if err = fn(); err == nil {
			return nil
		}
		if !isRetryable(err) {
			return err
		}
		if i < attempts-1 {
			log.Warn().Msgf("attempt %d/%d failed: %v; retrying in %s", i+1, attempts, err, backoff)
			time.Sleep(backoff)
			backoff *= 2
		}
	}
	return err
}

// UploadToSwift uploads localPath to containerName under targetPath. The connection is
// re-authenticated if its token has expired, and the container is created if missing.
//
// checkHash=true plus the precomputed md5 makes the client verify the uploaded body against
// the server's ETag, giving the same integrity guarantee as the python `etag=` argument.
func UploadToSwift(conn *swift.Connection, containerName, localPath, targetPath string) error {
	if conn == nil {
		return errors.New("swift connection is nil")
	}

	stat, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("failed to stat local file %s: %w", localPath, err)
	}

	md5sum, err := checkFileMD5(localPath)
	if err != nil {
		return fmt.Errorf("failed to calculate MD5 checksum for %s: %w", localPath, err)
	}
	log.Debug().Msgf("MD5 checksum of %s: %s (%d bytes)", localPath, md5sum, stat.Size())

	// Without an explicit Content-Length the client falls back to Transfer-Encoding: chunked,
	// which some Swift proxies reject outright. python-swiftclient always sends the length for
	// a file upload, so do the same.
	putHeaders := swift.Headers{"Content-Length": strconv.FormatInt(stat.Size(), 10)}

	ctx := context.Background()
	var headers swift.Headers

	err = withRetry(3, func() error {
		if !conn.Authenticated() {
			if err := conn.Authenticate(ctx); err != nil {
				return fmt.Errorf("swift auth: %w", err)
			}
		}

		// Idempotent: succeeds whether or not the container already exists.
		if err := conn.ContainerCreate(ctx, containerName, nil); err != nil {
			return fmt.Errorf("ensure container %q: %w", containerName, err)
		}

		file, err := os.Open(localPath)
		if err != nil {
			return fmt.Errorf("failed to open local file %s: %w", localPath, err)
		}
		defer file.Close()

		headers, err = conn.ObjectPut(ctx, containerName, targetPath, file, true, md5sum, "", putHeaders)
		if err != nil {
			return fmt.Errorf("put object %q: %w", targetPath, err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to upload file %s to Swift: %w", localPath, err)
	}

	log.Debug().Msgf("Successfully uploaded %s to container %s as %s\nHeaders: %v\n", localPath, containerName, targetPath, headers)
	return nil
}
