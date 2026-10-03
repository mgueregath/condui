package sftp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/pkg/sftp"
)

func DownloadFile(ctx context.Context, client *sftp.Client, remotePath, localPath string, progress func(int64) io.Writer) (err error) {
	if err = ctx.Err(); err != nil {
		return err
	}
	src, err := client.Open(remotePath)
	if err != nil {
		return err
	}
	sourceClosed := false
	defer func() {
		if !sourceClosed {
			_ = src.Close()
		}
	}()
	stat, err := src.Stat()
	if err != nil {
		return err
	}
	dst, err := os.CreateTemp(filepath.Dir(localPath), "."+filepath.Base(localPath)+".*.part")
	if err != nil {
		return err
	}
	tempPath := dst.Name()
	committed := false
	defer func() {
		if !committed {
			if cleanupErr := os.Remove(tempPath); cleanupErr != nil && !os.IsNotExist(cleanupErr) {
				err = errors.Join(err, fmt.Errorf("remove temporary download: %w", cleanupErr))
			}
		}
	}()
	var writer io.Writer = dst
	if progress != nil {
		if reporter := progress(stat.Size()); reporter != nil {
			writer = io.MultiWriter(dst, reporter)
		}
	}
	var transferred int64
	transferred, err = src.WriteTo(transferWriter{ctx: ctx, writer: writer})
	err = errors.Join(err, dst.Close(), src.Close())
	sourceClosed = true
	if err == nil && transferred != stat.Size() {
		err = fmt.Errorf("source size changed during transfer")
	}
	if err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	err = os.Rename(tempPath, localPath)
	committed = err == nil
	return err
}
