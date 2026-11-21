package meme

import "time"

const maxPresignTTL = 15 * time.Minute

const (
	defaultLimit       = 20
	maxLimit           = 100
	maxUploadSizeBytes = 10 * 1024 * 1024 // 10 MB batas ukuran gambar dasar
	maxResultSizeBytes = 10 << 20         // 10 MB batas ukuran gambar hasil render
)
