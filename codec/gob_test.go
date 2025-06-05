package codec

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGobCodec_ReadWrite(t *testing.T) {
	// 使用 bytes.Buffer 作为双向通信缓冲区
	var buf bytes.Buffer

	// 创建两个独立的编解码器，共享同一个缓冲区
	serverCodec := NewGobCodec(struct {
		io.Reader
		io.Writer
		io.Closer
	}{
		Reader: &buf,
		Writer: &buf,
		Closer: io.NopCloser(nil),
	}).(*GobCodec)

	clientCodec := NewGobCodec(struct {
		io.Reader
		io.Writer
		io.Closer
	}{
		Reader: &buf,
		Writer: &buf,
		Closer: io.NopCloser(nil),
	}).(*GobCodec)

	// 测试数据
	type TestBody struct {
		Name string
		Age  int
	}

	testHeader := &Header{
		ServiceMethod: "Test.Method",
		Seq:           12345,
		Error:         "",
	}

	testBody := &TestBody{
		Name: "Alice",
		Age:  30,
	}

	t.Run("Write and Read", func(t *testing.T) {
		// 重置缓冲区
		buf.Reset()

		// 客户端写入
		err := clientCodec.Write(testHeader, testBody)
		require.NoError(t, err, "Write should not error")
		require.NoError(t, clientCodec.buf.Flush(), "Flush should not error")

		// 服务端读取
		var h Header
		err = serverCodec.ReadHeader(&h)
		require.NoError(t, err, "ReadHeader should not error")

		assert.Equal(t, testHeader.ServiceMethod, h.ServiceMethod, "ServiceMethod should match")
		assert.Equal(t, testHeader.Seq, h.Seq, "Seq should match")
		assert.Equal(t, testHeader.Error, h.Error, "Error should match")

		var body TestBody
		err = serverCodec.ReadBody(&body)
		require.NoError(t, err, "ReadBody should not error")

		assert.Equal(t, testBody.Name, body.Name, "Body Name should match")
		assert.Equal(t, testBody.Age, body.Age, "Body Age should match")
	})
}
