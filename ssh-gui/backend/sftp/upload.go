package sftp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"

	"github.com/google/uuid"
	"github.com/pkg/sftp"
)

func UploadFile(ctx context.Context, client *sftp.Client, localPath, remotePath string, progress func(int64) io.Writer) (err error) {
	if err = ctx.Err(); err != nil {
		return err
	}
	src, err := os.Open(localPath)
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
	if !stat.Mode().IsRegular() {
		return fmt.Errorf("upload source must be a regular file")
	}
	tempPath := path.Join(path.Dir(remotePath), "."+path.Base(remotePath)+"."+uuid.NewString()+".part")
	dst, err := client.OpenFile(tempPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			if cleanupErr := client.Remove(tempPath); cleanupErr != nil && !os.IsNotExist(cleanupErr) {
				err = errors.Join(err, fmt.Errorf("remove temporary upload: %w", cleanupErr))
			}
		}
	}()
	var reader io.Reader = src
	if progress != nil {
		if writer := progress(stat.Size()); writer != nil {
			reader = io.TeeReader(src, writer)
		}
	}
	var transferred int64
	transferred, err = dst.ReadFromWithConcurrency(transferReader{ctx: ctx, reader: reader}, 32)
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
	if _, supported := client.HasExtension("posix-rename@openssh.com"); supported {
		err = client.PosixRename(tempPath, remotePath)
	} else {
		// Never delete the destination to work around unsupported atomic replacement.
		err = client.Rename(tempPath, remotePath)
	}
	committed = err == nil
	return err
}
