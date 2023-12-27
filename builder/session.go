package builder

import (
	"context"

	"dagger.io/dagger"
	"platform.prodigy9.co/internal/plog"
)

type Session struct {
	ctx    context.Context
	client *dagger.Client
}

func NewSession(ctx context.Context) (*Session, error) {
	client, err := dagger.Connect(ctx, dagger.WithLogOutput(plog.OutputForDagger()))
	if err != nil {
		return nil, err
	} else {
		return &Session{ctx: ctx, client: client}, nil
	}
}

func (s *Session) Client() *dagger.Client {
	return s.client
}
func (s *Session) Context() context.Context {
	return s.ctx
}
func (s *Session) JobContext(job *Job) (context.Context, context.CancelFunc) {
	return context.WithTimeout(s.ctx, job.Timeout)
}

func (s *Session) Close() error {
	return s.client.Close()
}
