package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"ingestion-go/internal/adapters/secondary"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
)

type OtaService struct {
	repo     *secondary.PostgresRepo
	s3Client *s3.S3
	mqtt     mqtt.Client
	bucket   string
}

func NewOtaService(repo *secondary.PostgresRepo) *OtaService {
	// S3 Config
	endpoint := os.Getenv("MG_BACKEND_OBJECT_STORAGE_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://seaweedfs-s3:8333"
	}
	accessKey := os.Getenv("MG_BACKEND_OBJECT_STORAGE_ACCESS_KEY")
	secretKey := os.Getenv("MG_BACKEND_OBJECT_STORAGE_SECRET_KEY")

	s3Config := &aws.Config{
		Credentials:      credentials.NewStaticCredentials(accessKey, secretKey, ""),
		Endpoint:         aws.String(endpoint),
		Region:           aws.String("us-east-1"), // Minio/Seaweed default
		S3ForcePathStyle: aws.Bool(true),
	}
	sess, _ := session.NewSession(s3Config)

	// MQTT (Standalone or Shared? Ideally Shared. For now Standalone client or wiring injection)
	// We will create a new client like ShadowService for V1 robustness
	// But ideally we inject MqttHandler's client.
	// Re-creating client is safe enough.

	broker := os.Getenv("MQTT_HOST")
	port := os.Getenv("MQTT_PORT")
	user := os.Getenv("SERVICE_MQTT_USERNAME")
	pass := os.Getenv("SERVICE_MQTT_PASSWORD")
	if broker == "" {
		broker = "localhost"
	}
	if port == "" {
		port = "1883"
	}

	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s:%s", broker, port))
	opts.SetClientID("go_ota_service_" + uuid.New().String()[:8])
	if user != "" {
		opts.SetUsername(user)
	}
	if pass != "" {
		opts.SetPassword(pass)
	}
	opts.SetAutoReconnect(true)

	client := mqtt.NewClient(opts)
	// Async Connect
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Printf("[OtaService] MQTT Connect Failed: %v", token.Error())
	}

	svc := &OtaService{
		repo:     repo,
		s3Client: s3.New(sess),
		mqtt:     client,
		bucket:   "ota-firmware",
	}

	svc.initBucket()
	return svc
}

func (s *OtaService) initBucket() {
	_, err := s.s3Client.HeadBucket(&s3.HeadBucketInput{Bucket: aws.String(s.bucket)})
	if err != nil {
		log.Printf("[OtaService] Creating Bucket %s...", s.bucket)
		s.s3Client.CreateBucket(&s3.CreateBucketInput{Bucket: aws.String(s.bucket)})
	}
}

func (s *OtaService) UploadFirmware(filename string, data []byte) (string, error) {
	key := fmt.Sprintf("firmware/%d_%s", time.Now().Unix(), filename)

	_, err := s.s3Client.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(data),
		ACL:    aws.String("public-read"),
	})
	if err != nil {
		return "", err
	}

	endpoint := os.Getenv("MG_BACKEND_OBJECT_STORAGE_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://seaweedfs-s3:8333"
	}
	return fmt.Sprintf("%s/%s/%s", endpoint, s.bucket, key), nil
}

func (s *OtaService) StartCampaign(name, version, s3Url, checksum, projectType string) error {
	log.Printf("[OtaService] Starting Campaign %s (%s)", name, version)

	// 1. Find Devices
	// Need repo method. Using raw SQL fallback if repo method missing?
	// Using repo.GetDevicesByType (We need to verify if it exists, otherwise add it)
	devices, err := s.repo.GetDevicesByType(projectType)
	if err != nil {
		return err
	}

	if len(devices) == 0 {
		return fmt.Errorf("no devices found for type %s", projectType)
	}

	// 2. Blast Commands
	for _, dev := range devices {
		imei := dev["imei"].(string) // Map check
		projectId := dev["project_id"].(string)

		topic := fmt.Sprintf("channels/%s/ota/%s", projectId, imei)
		payload := map[string]interface{}{
			"msgid": uuid.New().String(),
			"cmd":   "OTA_UPDATE",
			"params": map[string]string{
				"version":  version,
				"url":      s3Url,
				"checksum": checksum,
			},
			"ts": time.Now().UnixMilli(),
		}

		data, _ := json.Marshal(payload)
		s.mqtt.Publish(topic, 1, false, data)
		log.Printf("   -> OTA Command Sent to %s", imei)
	}

	return nil
}
