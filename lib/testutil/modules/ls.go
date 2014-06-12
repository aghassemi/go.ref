package modules

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"veyron2/rt"
	"veyron2/services/mounttable"
)

type globtag int

type glob struct{ tag globtag }

const (
	ls globtag = iota
	lsat
	lsmt
)

func NewGlob() T {
	return &glob{tag: ls}
}

func NewGlobAt() T {
	return &glob{tag: lsat}
}
func NewGlobAtMT() T {
	return &glob{tag: lsmt}
}

func (g *glob) Help() string {
	switch g.tag {
	case ls:
		return `<glob pattern>
lists names using the local mountTable client's Glob method`
	case lsat:
		return `<name> <glob pattern>
list names using by invoking the Glob method on name (name.Glob)`
	case lsmt:
		return `<name> <glob pattern>
list names using the Glob method on the mountTable at name`
	default:
		return "unknown glob command"
	}
}

func (g *glob) Daemon() bool { return false }

func (g *glob) Run(args []string) (Variables, []string, Handle, error) {
	switch {
	case g.tag == ls && len(args) != 1:
		return nil, nil, nil, fmt.Errorf("wrong # args: %s", g.Help())
	case (g.tag == lsat || g.tag == lsmt) && len(args) != 2:
		return nil, nil, nil, fmt.Errorf("wrong # args: %s", g.Help())
	}
	var r []string
	var err error
	switch g.tag {
	case ls:
		r, err = lsUsingLocalMountTable(args[0])
	case lsat:
		r, err = lsUsingResolve(args[0], args[1])
	case lsmt:
		r, err = lsUsingResolveToMountTable(args[0], args[1])
	default:
		return nil, nil, nil, fmt.Errorf("unrecognised glob tag %v", g.tag)
	}
	if err != nil {
		return nil, nil, nil, err
	} else {
		vars := make(Variables)
		for i, v := range r {
			k := "R" + strconv.Itoa(i)
			vars.Update(k, v)
		}
		vars.Update("ALL", strings.Join(r, ","))
		return vars, r, nil, nil
	}
}

func lsUsingLocalMountTable(pattern string) ([]string, error) {
	lmt := rt.R().MountTable()
	ch, err := lmt.Glob(rt.R().NewContext(), pattern)
	if err != nil {
		return nil, fmt.Errorf("pattern %q: %v", pattern, err)
	}
	var reply []string
	for e := range ch {
		reply = append(reply, fmt.Sprintf("%q", e.Name))
	}
	return reply, nil
}

func lsUsingResolve(name, pattern string) ([]string, error) {
	mtpt, err := mounttable.BindGlobable(name)
	if err != nil {
		return []string{}, err
	}
	stream, err := mtpt.Glob(nil, pattern)
	if err != nil {
		return []string{}, err
	}
	var reply []string
	for {
		e, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return reply, err
		}
		reply = append(reply, fmt.Sprintf("%q", e.Name))
	}
	return reply, nil
}

func lsUsingResolveToMountTable(name, pattern string) ([]string, error) {
	lmt := rt.R().MountTable()
	servers, err := lmt.ResolveToMountTable(rt.R().NewContext(), name)
	if err != nil {
		return []string{}, err
	}
	if len(servers) == 0 {
		return []string{}, fmt.Errorf("failed to find a mounttable for %q", name)
	}
	return lsUsingResolve(servers[0], pattern)
}
