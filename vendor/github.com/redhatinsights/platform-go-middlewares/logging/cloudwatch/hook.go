package cloudwatch

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/sirupsen/logrus"
)

type Hook struct {
	svc               *cloudwatchlogs.CloudWatchLogs
	groupName         string
	streamName        string
	nextSequenceToken *string
	m                 sync.Mutex
	ch                chan *cloudwatchlogs.InputLogEvent
	flush             chan bool
	err               *error
}

func NewHookWithDuration(groupName, streamName string, cfg *aws.Config, batchFrequency time.Duration) (*Hook, error) {
	return NewBatchingHook(groupName, streamName, cfg, batchFrequency)
}

func NewHook(groupName, streamName string, cfg *aws.Config) (*Hook, error) {
	return NewBatchingHook(groupName, streamName, cfg, 0)
}

func (h *Hook) getOrCreateCloudWatchLogGroup() (*cloudwatchlogs.DescribeLogStreamsOutput, error) {
	resp, err := h.svc.DescribeLogStreams(&cloudwatchlogs.DescribeLogStreamsInput{
		LogGroupName:        aws.String(h.groupName),
		LogStreamNamePrefix: aws.String(h.streamName),
	})

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case cloudwatchlogs.ErrCodeResourceNotFoundException:
				_, err = h.svc.CreateLogGroup(&cloudwatchlogs.CreateLogGroupInput{
					LogGroupName: aws.String(h.groupName),
				})
				if err != nil {
					return nil, err
				}
				return h.getOrCreateCloudWatchLogGroup()
			default:
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	return resp, nil

}

func NewBatchingHook(groupName, streamName string, cfg *aws.Config, batchFrequency time.Duration) (*Hook, error) {
	h := &Hook{
		svc:        cloudwatchlogs.New(session.New(cfg)),
		groupName:  groupName,
		streamName: streamName,
		// unbuferred channel for flushing, calling flush method will block
		flush: make(chan bool, 0),
	}

	resp, err := h.getOrCreateCloudWatchLogGroup()
	if err != nil {
		return nil, err
	}

	if batchFrequency > 0 {
		h.ch = make(chan *cloudwatchlogs.InputLogEvent, 10000)
		go h.putBatches(h.flush, time.Tick(batchFrequency))
	}

	// grab the next sequence token
	if len(resp.LogStreams) > 0 {
		h.nextSequenceToken = resp.LogStreams[0].UploadSequenceToken
		return h, nil
	}

	// create stream if it doesn't exist. the next sequence token will be null
	_, err = h.svc.CreateLogStream(&cloudwatchlogs.CreateLogStreamInput{
		LogGroupName:  aws.String(groupName),
		LogStreamName: aws.String(streamName),
	})
	if err != nil {
		return nil, err
	}

	return h, nil
}

// Force flushing of currently stored messages
func (h *Hook) Flush() error {
	h.flush <- true
	if h.err != nil {
		return *h.err
	}
	return nil
}

func (h *Hook) Fire(entry *logrus.Entry) error {
	line, err := entry.String()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to read entry, %v", err)
		return err
	}

	switch entry.Level {
	case logrus.PanicLevel:
		fallthrough
	case logrus.FatalLevel:
		fallthrough
	case logrus.ErrorLevel:
		fallthrough
	case logrus.WarnLevel:
		fallthrough
	case logrus.InfoLevel:
		fallthrough
	case logrus.DebugLevel:
		_, err := h.Write([]byte(line))
		return err
	default:
		return nil
	}
}

func (h *Hook) putBatches(flush <-chan bool, ticker <-chan time.Time) {
	var batch []*cloudwatchlogs.InputLogEvent
	size := 0
	for {
		select {
		case p := <-h.ch:
			messageSize := len(*p.Message) + 26
			if size+messageSize >= 1048576 || len(batch) == 10000 {
				go h.sendBatch(batch)
				batch = nil
				size = 0
			}
			batch = append(batch, p)
			size += messageSize
		case <-flush:
			go h.sendBatch(batch)
			batch = nil
			size = 0
		case <-ticker:
			go h.sendBatch(batch)
			batch = nil
			size = 0
		}
	}
}

func (h *Hook) sendBatch(batch []*cloudwatchlogs.InputLogEvent) {
	h.m.Lock()

	if len(batch) == 0 {
		h.m.Unlock()
		return
	}
	params := &cloudwatchlogs.PutLogEventsInput{
		LogEvents:     batch,
		LogGroupName:  aws.String(h.groupName),
		LogStreamName: aws.String(h.streamName),
		SequenceToken: h.nextSequenceToken,
	}
	resp, err := h.svc.PutLogEvents(params)
	if err == nil {
		h.nextSequenceToken = resp.NextSequenceToken
		h.m.Unlock()
		return
	}

	h.err = &err
	if aerr, ok := err.(*cloudwatchlogs.InvalidSequenceTokenException); ok {
		h.nextSequenceToken = aerr.ExpectedSequenceToken
		h.m.Unlock()
		h.sendBatch(batch)
		return
	}
	
	h.m.Unlock()
}

func (h *Hook) Write(p []byte) (n int, err error) {
	event := &cloudwatchlogs.InputLogEvent{
		Message:   aws.String(string(p)),
		Timestamp: aws.Int64(int64(time.Nanosecond) * time.Now().UnixNano() / int64(time.Millisecond)),
	}

	if h.ch != nil {
		h.ch <- event
		if h.err != nil {
			lastErr := h.err
			h.err = nil
			return 0, fmt.Errorf("%v", *lastErr)
		}
		return len(p), nil
	}

	h.m.Lock()
	defer h.m.Unlock()

	params := &cloudwatchlogs.PutLogEventsInput{
		LogEvents:     []*cloudwatchlogs.InputLogEvent{event},
		LogGroupName:  aws.String(h.groupName),
		LogStreamName: aws.String(h.streamName),
		SequenceToken: h.nextSequenceToken,
	}
	resp, err := h.svc.PutLogEvents(params)
	if err != nil {
		return 0, err
	}

	h.nextSequenceToken = resp.NextSequenceToken

	return len(p), nil
}

func (h *Hook) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.PanicLevel,
		logrus.FatalLevel,
		logrus.ErrorLevel,
		logrus.WarnLevel,
		logrus.InfoLevel,
		logrus.DebugLevel,
	}
}

// WriterHook is a hook that just outputs to an io.Writer.
// This is useful because our formatter outputs the file
// and line where it was called, and the callstack for a hook
// is different from the callstack for just writing to logrus.Logger.Out.
type WriterHook struct {
	w io.Writer
}

func NewWriterHook(w io.Writer) *WriterHook {
	return &WriterHook{w: w}
}

func (h *WriterHook) Fire(entry *logrus.Entry) error {
	line, err := entry.String()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to read entry, %v", err)
		return err
	}

	_, err = h.w.Write([]byte(line))
	return err
}

func (h *WriterHook) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.PanicLevel,
		logrus.FatalLevel,
		logrus.ErrorLevel,
		logrus.WarnLevel,
		logrus.InfoLevel,
		logrus.DebugLevel,
	}
}
