// Runs the servicerunner binary and checks that it outputs a JSON line to
// stdout with the expected variables.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path"
	"testing"
)

func check(t *testing.T, err error) {
	if err != nil {
		t.Fatal(err)
	}
}

func TestMain(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "servicerunner_test")
	check(t, err)

	bin := path.Join(tmpdir, "servicerunner")
	fmt.Println("Building", bin)
	check(t, exec.Command("veyron", "go", "build", "-o", bin, "veyron.io/veyron/veyron/tools/servicerunner").Run())

	cmd := exec.Command(bin)
	stdout, err := cmd.StdoutPipe()
	check(t, err)
	check(t, cmd.Start())

	line, err := bufio.NewReader(stdout).ReadBytes('\n')
	check(t, err)
	vars := map[string]string{}
	check(t, json.Unmarshal(line, &vars))
	fmt.Println(vars)
	for _, name := range []string{"VEYRON_CREDENTIALS", "MT_NAME", "PROXY_ADDR", "WSPR_ADDR"} {
		if _, ok := vars[name]; !ok {
			t.Error("Missing", name)
		}
	}

	check(t, cmd.Process.Kill())
}
