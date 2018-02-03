package tightbeam

import (
	"bytes"
	"errors"
	s "strings"
)

var tagEscapeDecodeMap = map[rune]rune{
	':':  ';',
	's':  ' ',
	'\\': '\\',
	'r':  '\r',
	'n':  '\n',
}

var tagEscapeEncodeMap = map[rune]string{
	';':  "\\:",
	' ':  "\\s",
	'\\': "\\\\",
	'\r': "\\r",
	'\n': "\\n",
}

var (
	ErrorZeroLengthMessage = errors.New("irc: Can't parse a zero length message")

	ErrorNothingAfterPrefix = errors.New("irc: No data after prefix")

	ErrorNoDataAfterTags = errors.New("irc: No data after tags")

	ErrorNoCommand = errors.New("irc: No command message")
)

type TagVal string

func ParseTagVal(v string) TagVal {
	ret := &bytes.Buffer{}

	input := bytes.NewBufferString(v)

	for {
		c, _, err := input.ReadRune()
		if err != nil {
			break
		}

		if c == '\\' {
			c2, _, err := input.ReadRune()
			if err != nil {
				break
			}
			if rep, ok := tagEscapeDecodeMap[c2]; ok {
				ret.WriteRune(rep)
			} else {
				ret.WriteRune(c2)
			}
		} else {
			ret.WriteRune(c)

		}
	}

	return TagVal(ret.String())
}

func (v TagVal) Encode() string {
	ret := &bytes.Buffer{}

	for _, c := range v {
		if rep, ok := tagEscapeEncodeMap[c]; ok {
			ret.WriteString(rep)
		} else {
			ret.WriteRune(c)
		}
	}

	return ret.String()
}

type Tags map[string]TagVal

func ParseTags(line string) Tags {
	ret := Tags{}

	tags := s.Split(line, ";")
	for _, tag := range tags {
		parts := s.SplitN(tag, "=", 2)
		if len(parts) > 2 {
			ret[parts[0]] = ""
			continue
		}

		ret[parts[0]] = ParseTagVal(parts[1])
	}

	return ret
}

func (t Tags) GetTag(key string) (string, bool) {
	ret, ok := t[key]
	return string(ret), ok
}

func (t Tags) Copy() Tags {
	ret := Tags{}

	for k, v := range t {
		ret[k] = v
	}

	return ret
}

func (t Tags) String() string {
	buf := &bytes.Buffer{}

	for k, v := range t {
		buf.WriteByte(';')
		buf.WriteString(k)
		if v != "" {
			buf.WriteByte('=')
			buf.WriteString(v.Encode())
		}
	}

	buf.ReadByte()

	return buf.String()
}

type Prefix struct {
	Name string
	User string
	Host string
}

func ParsePrefix(line string) *Prefix {
	id := &Prefix{
		Name: line,
	}

	uh := s.SplitN(id.Name, "@", 2)
	if len(uh) == 2 {
		id.Name, id.Host = uh[0], uh[1]
	}

	nu := s.SplitN(id.Name, "!", 2)
	if len(nu) == 2 {
		id.Name, id.User = nu[0], nu[1]
	}

	return id
}

func (p *Prefix) Copy() *Prefix {
	if p == nil {
		return nil
	}

	newPrefix := &Prefix{}

	*newPrefix = *p

	return newPrefix
}

func (p *Prefix) String() string {
	buf := &bytes.Buffer{}

	buf.WriteString(p.Name)

	if p.User != "" {
		buf.WriteString("!")
		buf.WriteString(p.User)
	}

	if p.Host != "" {
		buf.WriteString("@")
		buf.WriteString(p.Host)
	}

	return buf.String()
}

type Message struct {
	Tags
	*Prefix
	Command string
	Params  []string
}

func MustParseMessage(line string) *Message {
	m, err := ParseMessage(line)
	if err != nil {
		panic(err.Error())
	}
	return m
}

func ParseMessage(line string) (*Message, error) {
	line = s.TrimRight(line, "\r\n")
	if len(line) == 0 {
		return nil, ErrorZeroLengthMessage
	}

	c := &Message{
		Tags:   Tags{},
		Prefix: &Prefix{},
	}

	if line[0] == '@' {
		split := s.SplitN(line, " ", 2)
		if len(split) < 2 {
			return nil, ErrorNoDataAfterTags
		}

		c.Tags = ParseTags(split[0][1:])
		line = split[1]
	}

	if line[0] == ':' {
		split := s.SplitN(line, " ", 2)
		if len(split) < 2 {
			return nil, ErrorNothingAfterPrefix
		}

		c.Prefix = ParsePrefix(split[0][1:])
		line = split[1]
	}

	split := s.SplitN(line, " :", 2)
	c.Params = s.FieldsFunc(split[0], func(r rune) bool {
		return r == ' '
	})

	if len(c.Params) == 0 {
		return nil, ErrorNoCommand
	}

	if len(split) == 2 {
		c.Params = append(c.Params, split[1])
	}

	c.Command = s.ToUpper(c.Params[0])
	c.Params = c.Params[1:]

	if len(c.Params) == 0 {
		c.Params = nil
	}

	return c, nil
}

func (m *Message) Trailing() string {
	if len(m.Params) < 1 {
		return ""
	}

	return m.Params[len(m.Params)-1]
}

func (m *Message) Copy() *Message {
	newMessage := &Message{}

	*newMessage = *m

	newMessage.Tags = m.Tags.Copy()

	newMessage.Prefix = m.Prefix.Copy()

	newMessage.Params = append(make([]string, 0, len(m.Params)), m.Params...)

	if len(newMessage.Params) == 0 {
		newMessage.Params = nil
	}

	return newMessage
}

func (m *Message) String() string {
	buf := bytes.Buffer{}

	if len(m.Tags) > 0 {
		buf.WriteByte('@')
		buf.WriteString(m.Tags.String())
		buf.WriteByte(' ')
	}

	if m.Prefix != nil && m.Prefix.Name != "" {
		buf.WriteByte(':')
		buf.WriteString(m.Prefix.String())
		buf.WriteByte(' ')
	}

	buf.WriteString(m.Command)

	if len(m.Params) > 0 {
		args := m.Params[:len(m.Params)-1]
		trailing := m.Params[len(m.Params)-1]

		if len(args) > 0 {
			buf.WriteByte(' ')
			buf.WriteString(s.Join(args, " "))
		}

		if len(trailing) == 0 || s.ContainsRune(trailing, ' ') || trailing[0] == ':' {
			buf.WriteString(" :")
		} else {
			buf.WriteString(" ")
		}

		buf.WriteString(trailing)
	}

	return buf.String()
}
