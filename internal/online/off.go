package online

import "context"

// Off is the client with no consent: every call returns ErrOff at once.
var Off Client = off{}

type off struct{}

func (off) Status(context.Context) (Status, error) {
	return Status{}, ErrOff
}

func (off) PostDaily(context.Context, string, int, bool, int) error {
	return ErrOff
}

func (off) Daily(context.Context, string, int) (DailyCount, error) {
	return DailyCount{}, ErrOff
}
