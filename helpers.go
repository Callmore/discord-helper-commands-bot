package main

import (
	"fmt"
	"time"
)

// Discord colour constants
const (
	DiscordBlurple = 0x5865F2
	DiscordGreen   = 0x57F287
	DiscordYellow  = 0xFEE75C
	DiscordFuscha  = 0xEB459E
	DiscordRed     = 0xED4245
	DiscordWhite   = 0xFFFFFF
	DiscordBlack   = 0x000000
)

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func ptr[T interface{}](val T) *T {
	return &val
}

type TimestampFormat string

const (
	TimestampDefault       TimestampFormat = ""
	TimestampShortTime     TimestampFormat = "t"
	TimestampLongTime      TimestampFormat = "T"
	TimestampShortDate     TimestampFormat = "d"
	TimestampLongDate      TimestampFormat = "D"
	TimestampShortDateTime TimestampFormat = "f"
	TimestampLongDateTime  TimestampFormat = "F"
	TimestampRelative      TimestampFormat = "R"
)

func Timestamp(t time.Time, format TimestampFormat) string {
	if format == TimestampDefault {
		return fmt.Sprintf("<t:%d>", t.Unix())
	}
	return fmt.Sprintf("<t:%d:%s>", t.Unix(), format)
}

func parseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, nil
	}

	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, err
	}

	if d < 0 {
		return 0, fmt.Errorf("duration must be positive")
	}

	return d, nil
}
