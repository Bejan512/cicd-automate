package prompt

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// Prompter handles interactive CLI prompts with non-interactive fallback.
type Prompter struct {
	In             *bufio.Reader
	Out            io.Writer
	NonInteractive bool
}

// New creates a Prompter reading from stdin and writing to stdout.
func New(nonInteractive bool) *Prompter {
	return &Prompter{
		In:             bufio.NewReader(os.Stdin),
		Out:            os.Stdout,
		NonInteractive: nonInteractive,
	}
}

// Input asks for a free-form string value with an optional default.
func (p *Prompter) Input(label, defaultVal string) (string, error) {
	if p.NonInteractive {
		return defaultVal, nil
	}
	if defaultVal != "" {
		fmt.Fprintf(p.Out, "? %s [%s]: ", label, defaultVal)
	} else {
		fmt.Fprintf(p.Out, "? %s: ", label)
	}

	line, err := p.In.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	val := strings.TrimSpace(line)
	if val == "" {
		return defaultVal, nil
	}
	return val, nil
}

// Confirm asks a yes/no question with a default answer.
func (p *Prompter) Confirm(label string, defaultYes bool) (bool, error) {
	if p.NonInteractive {
		return defaultYes, nil
	}
	hint := "y/N"
	if defaultYes {
		hint = "Y/n"
	}
	fmt.Fprintf(p.Out, "? %s (%s): ", label, hint)

	line, err := p.In.ReadString('\n')
	if err != nil && err != io.EOF {
		return false, err
	}
	ans := strings.ToLower(strings.TrimSpace(line))
	if ans == "" {
		return defaultYes, nil
	}
	return ans == "y" || ans == "yes", nil
}

// Select prompts the user to pick one option from options.
func (p *Prompter) Select(label string, options []string, defaultVal string) (string, error) {
	if p.NonInteractive || len(options) == 0 {
		if defaultVal != "" {
			return defaultVal, nil
		}
		if len(options) > 0 {
			return options[0], nil
		}
		return "", nil
	}

	defaultIdx := 0
	for i, opt := range options {
		if opt == defaultVal {
			defaultIdx = i
			break
		}
	}

	fmt.Fprintf(p.Out, "? %s:\n", label)
	for i, opt := range options {
		marker := " "
		if i == defaultIdx {
			marker = "*"
		}
		fmt.Fprintf(p.Out, "  %s %d) %s\n", marker, i+1, opt)
	}
	fmt.Fprintf(p.Out, "  Choose [1-%d] (default %d): ", len(options), defaultIdx+1)

	line, err := p.In.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	ans := strings.TrimSpace(line)
	if ans == "" {
		return options[defaultIdx], nil
	}
	if n, err := strconv.Atoi(ans); err == nil && n >= 1 && n <= len(options) {
		return options[n-1], nil
	}
	// Also allow typing the option name directly.
	for _, opt := range options {
		if strings.EqualFold(opt, ans) {
			return opt, nil
		}
	}
	return options[defaultIdx], nil
}
