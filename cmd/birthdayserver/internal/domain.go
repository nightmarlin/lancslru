package internal

import (
	"strings"
	"time"
)

type Name string

func (n Name) String() string  { return string(n) }
func (n Name) Normalize() Name { return Name(strings.ToLower(n.String())) }

type Birthday time.Time

func (b Birthday) String() string { return time.Time(b).Format("2006-01-02") }
