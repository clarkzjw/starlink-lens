package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	swift "github.com/ncw/swift/v2"
)

// func sync_data() {
// 	cmd := exec.Command(SSHPassPath,
// 		"-p", SyncKey,
// 		"rsync",
// 		"-4",
// 		"--remove-source-files",
// 		"-e", "'ssh -o StrictHostKeychecking=no'",
// 		"--exclude=*.txt",
// 		"--exclude=*.json",
// 		"-a",
// 		"-v",
// 		"-z",
// 		path.Join(DataDir, "*"),
// 		fmt.Sprintf("%s@%s:%s", SyncUser, SyncServer, path.Join(SyncPath, ClientName)))

// 	shellCmd := exec.Command("bash", "-c", cmd.String())
// 	log.Println(shellCmd.String())

// 	shellCmd.Stdout = os.Stdout
// 	shellCmd.Stderr = os.Stderr

// 	if err := shellCmd.Run(); err != nil {
// 		log.Println(err)
// 	}

// 	if NotifyURL != "" {
// 		cmd := exec.Command("curl", "--retry", "3", "-4", "-X", "GET", NotifyURL)
// 		log.Println(cmd.String())
// 		cmd.Stdout = os.Stdout
// 		cmd.Stderr = os.Stderr

// 		if err := cmd.Run(); err != nil {
// 			log.Println(err)
// 		}
// 	}
// }

// type S3Client struct {
// 	*s3.Client
// }

// func NewS3Client() *S3Client {
// 	cfg := aws.Config{
// 		Region:       S3Region,
// 		BaseEndpoint: aws.String(S3Endpoint),
// 		Credentials:  aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(S3AccessKey, S3SecretKey, "")),
// 	}

// 	client := s3.NewFromConfig(cfg)

// 	return &S3Client{
// 		Client: client,
// 	}
// }

// func upload_to_s3(local_path string, remote_prefix string) {
// 	s3_client := NewS3Client()

// 	if _, err := os.Stat(local_path); os.IsNotExist(err) {
// 		log.Printf("File %s does not exist, skipping upload\n", local_path)
// 		return
// 	}

// 	remote_key := path.Base(local_path)

// 	if remote_prefix != "" {
// 		remote_key = path.Join(remote_prefix, remote_key)
// 	}

// 	log.Printf("Uploading %s to s3://%s/%s\n", local_path, S3BucketName, remote_key)

// 	f, err := os.Open(local_path)
// 	if err != nil {
// 		log.Println("Error opening file: ", err)
// 		return
// 	}
// 	defer f.Close()

// 	_, err = s3_client.PutObject(
// 		context.TODO(),
// 		&s3.PutObjectInput{
// 			Bucket: aws.String(S3BucketName),
// 			Key:    aws.String(remote_key),
// 			Body:   f,
// 		},
// 	)
// 	if err != nil {
// 		log.Println("Error uploading file: ", err)
// 		return
// 	}

// 	log.Printf("Successfully uploaded %s to s3://%s/%s\n", local_path, S3BucketName, remote_key)
// }

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

	log.Println("Swift containers:")
	for _, container := range containers {
		log.Println(" - ", container)
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
	fmt.Printf("MD5 checksum of %s: %s\n", localPath, md5sum)
	headers, err := conn.ObjectPut(context.Background(), containerName, targetPath, file, true, md5sum, "", nil)
	if err != nil {
		return fmt.Errorf("failed to upload file %s to Swift: %w", localPath, err)
	}
	fmt.Printf("Successfully uploaded %s to container %s as %s\nHeaders: %v\n", localPath, containerName, targetPath, headers)
	return nil
}
