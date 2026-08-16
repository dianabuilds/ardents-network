package main

import (
	"errors"
	"os"
	"strconv"
	"time"
)

func streamLifetime() (time.Duration, error) {
	value := os.Getenv("ARDENTS_STREAM_LIFETIME")
	if value == "" {
		return 15 * time.Second, nil
	}
	lifetime, err := time.ParseDuration(value)
	if err != nil || lifetime < 15*time.Second || lifetime > 30*time.Minute {
		return 0, errors.New("stream lifetime is outside its bound")
	}
	return lifetime, nil
}

func streamCounts(sendText, receiveText string) (int, int, error) {
	send64, sendErr := strconv.ParseInt(sendText, 10, 32)
	receive64, receiveErr := strconv.ParseInt(receiveText, 10, 32)
	if sendErr != nil || receiveErr != nil || send64 < 0 || receive64 < 0 || send64 > 256<<20 || receive64 > 256<<20 ||
		(send64 == 0 && receive64 == 0) {
		return 0, 0, errors.New("stream byte counts are outside their bound")
	}
	return int(send64), int(receive64), nil
}
