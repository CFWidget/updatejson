package util

import (
	"io"
	"os"
)

type TempFile struct {
	io.ReadCloser
	io.WriteCloser

	io.ReaderAt
	io.WriterAt

	file *os.File
}

func NewTempFile() (*TempFile, error) {
	f, e := os.CreateTemp("", "*")
	if e != nil {
		return nil, e
	}
	return &TempFile{file: f}, e
}

func (tf *TempFile) Close() error {
	defer os.Remove(tf.file.Name())
	return tf.file.Close()
}

func (tf *TempFile) Read(p []byte) (n int, err error) {
	return tf.file.Read(p)
}

func (tf *TempFile) Write(p []byte) (n int, err error) {
	return tf.file.Write(p)
}

func (tf *TempFile) ReadAt(p []byte, off int64) (n int, err error) {
	return tf.file.ReadAt(p, off)
}

func (tf *TempFile) WriteAt(p []byte, off int64) (n int, err error) {
	return tf.file.WriteAt(p, off)
}
