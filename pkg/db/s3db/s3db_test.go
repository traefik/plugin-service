package s3db

import (
	"context"
	"io"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/traefik/plugin-service/pkg/db"
)

type s3Mock struct {
	mock.Mock
	testFile string
}

func (_m *s3Mock) GetObject(ctx context.Context, input *s3.GetObjectInput, opts ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	_ret := _m.Called(ctx, input, opts)

	return _ret.Get(0).(*s3.GetObjectOutput), _ret.Error(1)
}

func (_m *s3Mock) OnGetObject() *mock.Call {
	// By default, empty array of data
	output := &s3.GetObjectOutput{Body: io.NopCloser(strings.NewReader("[]"))}
	call := _m.Mock.On("GetObject",
		mock.Anything,
		mock.MatchedBy(func(input *s3.GetObjectInput) bool {
			return input.Bucket != nil && input.Key != nil && *input.Bucket == "bucket" && *input.Key == "key"
		}),
		mock.Anything).Return(output, nil).Once()

	if _m.testFile != "" {
		data, err := os.Open(path.Join(path.Dir("."), "fixtures", _m.testFile))
		if err != nil {
			call.Panic(err.Error())
		}
		output.Body = data
	}

	return call
}

func newMockClient(testFile string) *s3Mock {
	client := &s3Mock{testFile: testFile}
	client.OnGetObject()

	return client
}

func TestS3DB_Create(t *testing.T) {
	client := newMockClient("")

	s3db, err := NewS3DB(context.Background(), client, "bucket", "key")
	require.NoError(t, err)

	_, err = s3db.Create(context.Background(), db.Plugin{})
	assert.Error(t, err)
	client.AssertExpectations(t)
}

func TestS3DB_Get(t *testing.T) {
	client := newMockClient("get.json")

	s3db, err := NewS3DB(context.Background(), client, "bucket", "key")
	require.NoError(t, err)
	assert.NotNil(t, s3db)

	plugin, err := s3db.Get(context.Background(), "123")
	require.NoError(t, err)
	assert.Equal(t, "github.com/test/test123", plugin.Name)
	client.AssertExpectations(t)
}

func TestS3DB_List(t *testing.T) {
	client := newMockClient("list.json")

	s3db, err := NewS3DB(context.Background(), client, "bucket", "key")
	require.NoError(t, err)
	assert.NotNil(t, s3db)

	plugins, _, err := s3db.List(context.Background(), db.Pagination{})
	require.NoError(t, err)

	assert.Len(t, plugins, 10)
	assert.Equal(t, plugins[0].Stars, 150)
	assert.Greater(t, plugins[0].Stars, plugins[9].Stars)
}

func TestS3DB_GetByName(t *testing.T) {
	client := newMockClient("getbyname.json")

	s3db, err := NewS3DB(context.Background(), client, "bucket", "key")
	require.NoError(t, err)
	assert.NotNil(t, s3db)

	// filter disabled
	plugin, err := s3db.GetByName(context.Background(), "plugin", true)
	require.NoError(t, err)
	assert.Equal(t, plugin.ID, "plugin-enabled")
	assert.Equal(t, plugin.Name, "plugin")
	assert.Equal(t, plugin.Disabled, false)

	// unfiltered
	plugin, err = s3db.GetByName(context.Background(), "plugin", false)
	require.NoError(t, err)
	assert.Equal(t, plugin.ID, "plugin-disabled")
	assert.Equal(t, plugin.Name, "plugin")
	assert.Equal(t, plugin.Disabled, true)

	// case-sensitivity
	plugin, err = s3db.GetByName(context.Background(), "PLUGIn__", false)
	require.NoError(t, err)
	assert.Equal(t, plugin.ID, "plugin-case-sensitive")
	assert.Equal(t, plugin.Name, "PluGin__")

}

func TestS3DB_SearchByDisplayName(t *testing.T) {
	client := newMockClient("search.json")

	s3db, err := NewS3DB(context.Background(), client, "bucket", "key")
	require.NoError(t, err)
	assert.NotNil(t, s3db)

	plugins, _, err := s3db.SearchByDisplayName(context.Background(), "sab", db.Pagination{})
	require.NoError(t, err)
	require.Len(t, plugins, 2)
	assert.Equal(t, plugins[0].DisplayName, "sablier")
	assert.Equal(t, plugins[0].Disabled, false)
	assert.Equal(t, plugins[1].DisplayName, "Disable GraphQL")
	assert.Equal(t, plugins[0].Disabled, false)

	plugins, _, err = s3db.SearchByDisplayName(context.Background(), "bas", db.Pagination{})
	require.NoError(t, err)
	require.Len(t, plugins, 0)

	plugins, _, err = s3db.SearchByDisplayName(context.Background(), "Gra[a-z]+", db.Pagination{})
	require.NoError(t, err)
	require.Len(t, plugins, 1)
	assert.Equal(t, plugins[0].DisplayName, "Disable GraphQL")
	assert.Equal(t, plugins[0].Disabled, false)

}

func TestS3DB_Unimplemented(t *testing.T) {
	client := newMockClient("")

	s3db, err := NewS3DB(context.Background(), client, "bucket", "key")
	require.NoError(t, err)

	err = s3db.Bootstrap()
	assert.NoError(t, err)

	err = s3db.Ping(context.Background())
	assert.NoError(t, err)

	err = s3db.Delete(context.Background(), "")
	assert.Error(t, err)

	_, err = s3db.Update(context.Background(), "", db.Plugin{})
	assert.Error(t, err)

	_, err = s3db.CreateHash(context.Background(), "", "", "")
	assert.Error(t, err)

	_, err = s3db.GetHashByName(context.Background(), "", "")
	assert.Error(t, err)
}
