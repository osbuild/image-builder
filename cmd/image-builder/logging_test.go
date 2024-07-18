package main

import (
	"context"
	"testing"

	"github.com/osbuild/image-builder/internal/common"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestFire(t *testing.T) {
	testHook := &ctxHook{}

	tests := []struct {
		entry      *logrus.Entry
		exp_before map[string]string
		exp_after  map[string]string
	}{
		// empty calls should not crash
		{&logrus.Entry{},
			map[string]string{},
			map[string]string{},
		},
		{&logrus.Entry{Context: context.Background(), Data: make(logrus.Fields)},
			map[string]string{},
			map[string]string{},
		},

		// Data should be added
		{&logrus.Entry{
			Context: common.WithRequestId(context.Background(), "Test"),
			Data:    make(logrus.Fields)},
			map[string]string{},
			map[string]string{"request_id": "Test"},
		},

		// valid DataCtx
		{&logrus.Entry{
			Context: common.WithRequestData(context.Background(),
				logrus.Fields{
					"method": "GET",
					"path":   "/"}),
			Data: make(logrus.Fields)},
			map[string]string{},
			map[string]string{"method": "GET"},
		},

		// invalid DataCtx
		{&logrus.Entry{
			Context: common.WithRequestData(context.Background(), nil),
			Data:    make(logrus.Fields)},
			map[string]string{},
			map[string]string{},
		},
	}

	for _, test := range tests {
		for k, v := range test.exp_before {
			require.Equal(t, v, test.entry.Data[k])
		}

		require.Equal(t, testHook.Fire(test.entry), nil)
		for k, v := range test.exp_after {
			require.Equal(t, v, test.entry.Data[k])
		}
	}

}
