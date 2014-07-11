package netconfig_test

import (
	"fmt"
	"log"

	"veyron/runtimes/google/lib/netconfig"
)

func ExampleNetConfigWatcher() {
	w, err := netconfig.NewNetConfigWatcher()
	if err != nil {
		log.Fatalf("oops: %s", err)
	}
	fmt.Println("Do something to your network. You should see one or more dings.")
	for {
		<-w.Channel()
		fmt.Println("ding")
	}
}
