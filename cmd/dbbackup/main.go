package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"academyprometheus/backend/config"
	"academyprometheus/backend/services"
)

func main() {
	cfg := config.Load()
	bucket := cfg.BackupR2Bucket
	accessKey := cfg.BackupR2AccessKeyID
	secretKey := cfg.BackupR2SecretAccessKey
	if bucket == "" {
		bucket = cfg.R2Bucket
	}
	if accessKey == "" {
		accessKey = cfg.R2AccessKeyID
	}
	if secretKey == "" {
		secretKey = cfg.R2SecretAccessKey
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	tmp, err := os.CreateTemp("", "prometheus-db-*.sql.gz")
	if err != nil {
		fatal("create temporary backup", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	gz := gzip.NewWriter(tmp)
	var stderr bytes.Buffer
	// #nosec G204 - mysqldump is a fixed executable and arguments are passed without a shell.
	command := exec.CommandContext(ctx, "mysqldump",
		"--host="+cfg.DBHost,
		"--port="+cfg.DBPort,
		"--user="+cfg.DBUser,
		"--single-transaction",
		"--quick",
		"--routines",
		"--triggers",
		"--events",
		"--hex-blob",
		"--set-gtid-purged=OFF",
		cfg.DBName,
	)
	command.Env = append(os.Environ(), "MYSQL_PWD="+cfg.DBPass)
	command.Stdout = gz
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		_ = gz.Close()
		_ = tmp.Close()
		fatal("mysqldump: "+stderr.String(), err)
	}
	if err := gz.Close(); err != nil {
		_ = tmp.Close()
		fatal("compress database backup", err)
	}
	if err := tmp.Close(); err != nil {
		fatal("close database backup", err)
	}

	// #nosec G304 - tmpName is created by os.CreateTemp in this function.
	file, err := os.Open(tmpName)
	if err != nil {
		fatal("open database backup", err)
	}
	defer func() { _ = file.Close() }()

	storage, err := services.NewR2Storage(ctx, cfg, bucket, accessKey, secretKey)
	if err != nil {
		fatal("configure R2 database backup", err)
	}
	key := filepath.ToSlash("database/" + time.Now().UTC().Format("2006/01/02/150405") + "-" + cfg.DBName + ".sql.gz")
	stored, err := storage.Put(ctx, services.PutObjectInput{Key: key, Body: file, ContentType: "application/gzip"})
	if err != nil {
		fatal("upload database backup", err)
	}
	fmt.Printf("database backup uploaded: %s (%d bytes)\n", stored.Key, stored.Size)
}

func fatal(action string, err error) {
	fmt.Fprintf(os.Stderr, "%s: %v\n", action, err)
	os.Exit(1)
}
