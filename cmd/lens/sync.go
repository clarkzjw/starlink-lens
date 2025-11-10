package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	swift "github.com/ncw/swift/v2"
)

func sync_data() {
	cmd := exec.Command(SSHPASS_PATH,
		"-p", SYNC_KEY,
		"rsync",
		"-4",
		"--remove-source-files",
		"-e", "'ssh -o StrictHostKeychecking=no'",
		"--exclude=*.txt",
		"--exclude=*.json",
		"-a",
		"-v",
		"-z",
		path.Join(DATA_DIR, "*"),
		fmt.Sprintf("%s@%s:%s", SYNC_USER, SYNC_SERVER, path.Join(SYNC_PATH, CLIENT_NAME)))

	shellCmd := exec.Command("bash", "-c", cmd.String())
	log.Println(shellCmd.String())

	shellCmd.Stdout = os.Stdout
	shellCmd.Stderr = os.Stderr

	if err := shellCmd.Run(); err != nil {
		log.Println(err)
	}

	if NOTIFY_URL != "" {
		cmd := exec.Command("curl", "--retry", "3", "-4", "-X", "GET", NOTIFY_URL)
		log.Println(cmd.String())
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			log.Println(err)
		}
	}
}

type S3Client struct {
	*s3.Client
}

func NewS3Client() *S3Client {
	cfg := aws.Config{
		Region:       S3_REGION,
		BaseEndpoint: aws.String(S3_ENDPOINT),
		Credentials:  aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(S3_ACCESS_KEY, S3_SECRET_KEY, "")),
	}

	client := s3.NewFromConfig(cfg)

	return &S3Client{
		Client: client,
	}
}

func upload_to_s3(local_path string, remote_prefix string) {
	s3_client := NewS3Client()

	if _, err := os.Stat(local_path); os.IsNotExist(err) {
		log.Printf("File %s does not exist, skipping upload\n", local_path)
		return
	}

	remote_key := path.Base(local_path)

	if remote_prefix != "" {
		remote_key = path.Join(remote_prefix, remote_key)
	}

	log.Printf("Uploading %s to s3://%s/%s\n", local_path, S3_BUCKET_NAME, remote_key)

	f, err := os.Open(local_path)
	if err != nil {
		log.Println("Error opening file: ", err)
		return
	}
	defer f.Close()

	_, err = s3_client.PutObject(
		context.TODO(),
		&s3.PutObjectInput{
			Bucket: aws.String(S3_BUCKET_NAME),
			Key:    aws.String(remote_key),
			Body:   f,
		},
	)
	if err != nil {
		log.Println("Error uploading file: ", err)
		return
	}

	log.Printf("Successfully uploaded %s to s3://%s/%s\n", local_path, S3_BUCKET_NAME, remote_key)
}

func new_swift_client(username, apiKey, authURL, domain, tenant string) (*swift.Connection, error) {
	conn := swift.Connection{
		UserName: username,
		ApiKey:   apiKey,
		AuthUrl:  authURL,
		Domain:   domain,
		Tenant:   tenant,
	}
	err := conn.Authenticate(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to authenticate Swift client: %v", err)
	}
	return &conn, nil
}

func test_swift_connection() error {
	conn, err := new_swift_client(SWIFT_USERNAME, SWIFT_APIKEY, SWIFT_AUTHURL, SWIFT_DOMAIN, SWIFT_TENANT)
	if err != nil {
		return err
	}

	containers, err := conn.ContainerNames(context.Background(), nil)
	if err != nil {
		return fmt.Errorf("failed to list Swift containers: %v", err)
	}

	log.Println("Swift containers:")
	for _, container := range containers {
		log.Println(" - ", container)
	}
	return nil
}

func upload_to_swift(conn *swift.Connection, containerName, localPath, targetPath string) error {
	if conn == nil {
		return fmt.Errorf("swift connection is nil")
	}

	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file %s: %v", localPath, err)
	}
	defer file.Close()

	md5sum, err := checkFileMD5(localPath)
	if err != nil {
		return fmt.Errorf("failed to calculate MD5 checksum for %s: %v", localPath, err)
	}
	fmt.Printf("MD5 checksum of %s: %s\n", localPath, md5sum)
	headers, err := conn.ObjectPut(context.Background(), containerName, targetPath, file, true, md5sum, "", nil)
	if err != nil {
		return fmt.Errorf("failed to upload file %s to Swift: %v", localPath, err)
	}
	fmt.Printf("Successfully uploaded %s to container %s as %s\nHeaders: %v", localPath, containerName, targetPath, headers)
	return nil
}
