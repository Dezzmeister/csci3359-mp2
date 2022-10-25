package common

import (
	"github.com/fatih/color"
)

const MESSAGE_CODE = uint16(0x0)
const ERROR_CODE = uint16(0x1)

/*
Limit the size of a username or message to prevent malicious clients from
eating memory on the server by spamming bogus requests to allocate a lot
of memory.
*/
var MAX_USERNAME_LENGTH = 32
var MAX_MESSAGE_LENGTH = 2048

var ColorOutput = color.Output
var NameColor = color.New(color.FgGreen).SprintFunc()
var MessageColor = color.New(color.FgBlue).SprintFunc()
var ErrorColor = color.New(color.FgRed).SprintFunc()
