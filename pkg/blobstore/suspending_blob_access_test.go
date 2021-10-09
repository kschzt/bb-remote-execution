package blobstore_test

import (
	"context"
	"io"
	"testing"

	"github.com/buildbarn/bb-remote-execution/internal/mock"
	"github.com/buildbarn/bb-remote-execution/pkg/blobstore"
	"github.com/buildbarn/bb-storage/pkg/blobstore/buffer"
	"github.com/buildbarn/bb-storage/pkg/digest"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestSuspendingBlobAccess(t *testing.T) {
	ctrl, ctx := gomock.WithContext(context.Background(), t)

	baseBlobAccess := mock.NewMockBlobAccess(ctrl)
	suspendable := mock.NewMockSuspendable(ctrl)
	blobAccess := blobstore.NewSuspendingBlobAccess(baseBlobAccess, suspendable)

	exampleDigest := digest.MustNewDigest("hello", "8b1a9953c4611296a827abf8c47804d7", 5)

	t.Run("Get", func(t *testing.T) {
		r := mock.NewMockReadCloser(ctrl)
		gomock.InOrder(
			suspendable.EXPECT().Suspend(),
			baseBlobAccess.EXPECT().Get(ctx, exampleDigest).
				Return(buffer.NewCASBufferFromReader(exampleDigest, r, buffer.UserProvided)))

		b := blobAccess.Get(ctx, exampleDigest)

		gomock.InOrder(
			r.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
				return copy(p, "Hello"), io.EOF
			}),
			r.EXPECT().Close(),
			suspendable.EXPECT().Resume())

		data, err := b.ToByteSlice(1000)
		require.NoError(t, err)
		require.Equal(t, []byte("Hello"), data)
	})

	t.Run("Put", func(t *testing.T) {
		gomock.InOrder(
			suspendable.EXPECT().Suspend(),
			baseBlobAccess.EXPECT().Put(ctx, exampleDigest, gomock.Any()).DoAndReturn(
				func(ctx context.Context, digest digest.Digest, b buffer.Buffer) error {
					data, err := b.ToByteSlice(1000)
					require.NoError(t, err)
					require.Equal(t, []byte("Hello"), data)
					return nil
				}),
			suspendable.EXPECT().Resume())

		require.NoError(t, blobAccess.Put(ctx, exampleDigest, buffer.NewValidatedBufferFromByteSlice([]byte("Hello"))))
	})

	t.Run("FindMissing", func(t *testing.T) {
		gomock.InOrder(
			suspendable.EXPECT().Suspend(),
			baseBlobAccess.EXPECT().FindMissing(ctx, digest.EmptySet).Return(digest.EmptySet, nil),
			suspendable.EXPECT().Resume())

		missing, err := blobAccess.FindMissing(ctx, digest.EmptySet)
		require.NoError(t, err)
		require.Equal(t, digest.EmptySet, missing)
	})
}