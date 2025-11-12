package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	swift "github.com/ncw/swift/v2"
	"github.com/phuslu/log"
)

func NewSwiftConn(username, apiKey, authURL, domain, tenant string) (*swift.Connection, error) {
	conn := swift.Connection{
		UserName: username,
		ApiKey:   apiKey,
		AuthUrl:  authURL,
		Domain:   domain,
		Tenant:   tenant,
	}
	err := conn.Authenticate(context.Background())
	if err != nil {
		return nil, errors.New("failed to authenticate Swift client: " + err.Error())
	}
	return &conn, nil
}

func TestSwiftConnection() error {
	conn, err := NewSwiftConn(SwiftUsername, SwiftAPIKey, SwiftAuthURL, SwiftDomain, SwiftTenant)
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

func UploadToSwift(conn *swift.Connection, containerName, localPath, targetPath string) error {
	if conn == nil {
		return errors.New("swift connection is nil")
	}

	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file %s: %w", localPath, err)
	}
	defer file.Close()

	md5sum, err := checkFileMD5(localPath)
	if err != nil {
		return fmt.Errorf("failed to calculate MD5 checksum for %s: %w", localPath, err)
	}
	log.Debug().Msgf("MD5 checksum of %s: %s", localPath, md5sum)
	headers, err := conn.ObjectPut(context.Background(), containerName, targetPath, file, true, md5sum, "", nil)
	if err != nil {
		return fmt.Errorf("failed to upload file %s to Swift: %w", localPath, err)
	}
	log.Debug().Msgf("Successfully uploaded %s to container %s as %s\nHeaders: %v\n", localPath, containerName, targetPath, headers)
	return nil
}
