package aws_s3

import (
	"bytes"
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

type S3Client struct {
	svc    *s3.S3
	bucket string
}

// NewS3Client initializes a new S3Client for Amazon S3
func NewS3Client(accessKey, secretKey, bucket, region string) (*S3Client, error) {
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String(region),
		Credentials: credentials.NewStaticCredentials(accessKey, secretKey, ""),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	svc := s3.New(sess)
	log.Println("S3: Successfully connected")
	return &S3Client{
		svc:    svc,
		bucket: bucket,
	}, nil
}

// Upload uploads a file to Amazon S3
func (sc *S3Client) Upload(data []byte, objectKey string) error {
	_, err := sc.svc.PutObject(&s3.PutObjectInput{
		Bucket:             aws.String(sc.bucket),
		Key:                aws.String(objectKey),
		Body:               bytes.NewReader(data),
		ContentLength:      aws.Int64(int64(len(data))),
		ContentType:        aws.String("image/jpg"),
		ContentDisposition: aws.String("attachment"),
		ACL:                aws.String("public-read"),
	})
	if err != nil {
		log.Println("S3:" + objectKey + " Failed upload" + err.Error())
		return fmt.Errorf("failed to upload data: %w", err)
	}
	log.Println("S3:" + objectKey + " Successfully uploaded")
	return nil
}

// Delete deletes a file from Amazon S3 given its URL
func (sc *S3Client) Delete(fileURL string, bucketURL string) error {
	// Extract the file name from the URL
	/////////////HACE FALTA CAMBIAR AQUI EL BUCKET URL
	parts := strings.Split(fileURL, bucketURL)
	if len(parts) != 2 {
		return fmt.Errorf("invalid file URL: %s", fileURL)
	}
	objectKey := parts[1]

	_, err := sc.svc.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(sc.bucket),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}

	// Wait until the object is deleted
	err = sc.svc.WaitUntilObjectNotExists(&s3.HeadObjectInput{
		Bucket: aws.String(sc.bucket),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return fmt.Errorf("error occurred while waiting for object to be deleted: %w", err)
	}
	log.Println("S3:" + fileURL + " Successfully deleted")
	return nil
}
