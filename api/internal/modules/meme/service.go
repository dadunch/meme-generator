package meme

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/nyxsr/meme-portal/api/internal/config"
	"github.com/nyxsr/meme-portal/api/internal/platform/token"
)

// Gate memeriksa apakah user berhak membuat meme (langganan aktif).
// Diimplementasikan oleh modul subscription lewat structural typing.
type Gate interface {
	CanCreate(ctx context.Context, userID string) error
}

type Service struct {
	repo    *Repository
	s3      *s3.Client
	bucket  string
	baseURL string // public base URL Garage web endpoint
	gate    Gate
}

func NewService(repo *Repository, s3Client *s3.Client, cfg config.StorageConfig, gate Gate) *Service {
	return &Service{repo: repo, s3: s3Client, bucket: cfg.Bucket, baseURL: cfg.PublicBaseURL, gate: gate}
}

// PrepareUpload membuat record meme (pending) dan mengembalikan presigned PUT URL
// untuk mengunggah gambar dasar langsung ke S3/Garage.
func (s *Service) PrepareUpload(ctx context.Context, userID, title, ext, contentType string, size int64) (UploadResponse, error) {
	if err := s.gate.CanCreate(ctx, userID); err != nil {
		return UploadResponse{}, err
	}
	mime := mimeFromExt(ext)
	if mime == "" {
		return UploadResponse{}, ErrInvalidFile
	}
	if size <= 0 {
		return UploadResponse{}, ErrInvalidFile
	}
	if size > maxUploadSizeBytes {
		return UploadResponse{}, ErrFileTooLarge
	}

	objectKey := fmt.Sprintf("base/%s%s", token.RandomHex(12), strings.ToLower(ext))

	m, err := s.repo.Create(ctx, Meme{
		UserID: userID, Title: title,
		BaseImageURL: s.publicURL(objectKey),
	})
	if err != nil {
		return UploadResponse{}, err
	}

	presign := s3.NewPresignClient(s.s3)
	req, err := presign.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:        &s.bucket,
		Key:           &objectKey,
		ContentType:   &mime,
		ContentLength: &size,
	}, s3.WithPresignExpires(maxPresignTTL))
	if err != nil {
		return UploadResponse{}, fmt.Errorf("meme service: presign gagal: %w", err)
	}
	return UploadResponse{ID: m.ID, UploadURL: req.URL, BaseURL: m.BaseImageURL}, nil
}

// ResultUpload mengembalikan presigned PUT URL untuk gambar hasil render
// (canvas) milik meme <id>; complete kemudian menyimpan URL publiknya.
func (s *Service) ResultUpload(ctx context.Context, userID, id string, size int64) (UploadResponse, error) {
	if err := s.gate.CanCreate(ctx, userID); err != nil {
		return UploadResponse{}, err
	}
	if _, err := s.repo.GetByID(ctx, id, userID); err != nil {
		return UploadResponse{}, err
	}
	if size <= 0 {
		return UploadResponse{}, ErrInvalidFile
	}
	if size > maxResultSizeBytes {
		return UploadResponse{}, ErrFileTooLarge
	}
	objectKey := fmt.Sprintf("result/%s.png", id)

	presign := s3.NewPresignClient(s.s3)
	req, err := presign.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:        &s.bucket,
		Key:           &objectKey,
		ContentType:   aws.String("image/png"),
		ContentLength: &size,
	}, s3.WithPresignExpires(maxPresignTTL))
	if err != nil {
		return UploadResponse{}, fmt.Errorf("meme service: presign gagal: %w", err)
	}
	return UploadResponse{ID: id, UploadURL: req.URL, BaseURL: s.publicURL(objectKey)}, nil
}

func (s *Service) Complete(ctx context.Context, userID, id string) (Meme, error) {
    if err := s.gate.CanCreate(ctx, userID); err != nil { return Meme{}, err }
    if _, err := s.repo.GetByID(ctx, id, userID); err != nil { return Meme{}, err }

    key := fmt.Sprintf("result/%s.png", id)
    drop := func() {
        _, _ = s.s3.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: &s.bucket, Key: &key})
    }

    head, err := s.s3.HeadObject(ctx, &s3.HeadObjectInput{Bucket: &s.bucket, Key: &key})
    if err != nil { return Meme{}, ErrInvalidFile }
    if aws.ToInt64(head.ContentLength) > maxResultSizeBytes {
        drop()
        return Meme{}, ErrFileTooLarge
    }

    obj, err := s.s3.GetObject(ctx, &s3.GetObjectInput{
        Bucket: &s.bucket, Key: &key, Range: aws.String("bytes=0-511"),
    })
    if err != nil { return Meme{}, ErrInvalidFile }
    defer obj.Body.Close()
    buf, _ := io.ReadAll(obj.Body)
    if http.DetectContentType(buf) != "image/png" {
        drop()
        return Meme{}, ErrInvalidFile
    }
    return s.repo.SetResultURL(ctx, id, userID, s.publicURL(key))
}

func (s *Service) List(ctx context.Context, userID string, limit, offset int32) (ListResponse, error) {
	items, total, err := s.repo.ListByUser(ctx, userID, limit, offset)
	if err != nil {
		return ListResponse{}, err
	}
	if items == nil {
		items = []Meme{}
	}
	return ListResponse{Items: items, Total: total}, nil
}

func (s *Service) Delete(ctx context.Context, userID, id string) error {
	return s.repo.Delete(ctx, id, userID)
}

func (s *Service) publicURL(key string) string {
	if s.baseURL == "" {
		return key
	}
	return strings.TrimRight(s.baseURL, "/") + "/" + path.Clean(key)
}

func allowedExt(ext string) bool {
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp":
		return true
	}
	return false
}

func mimeFromExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	}
	return ""
}
