package main

import (
	"fmt"
	"os"

	"fridare-gui/internal/rebuild"
)

func main() {
	path := "Dockerfile"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}
	mirror := ""
	if len(os.Args) > 2 {
		mirror = os.Args[2]
	}
	body := rebuild.DockerfileSkeletonForMirror(mirror)
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		panic(err)
	}
	fmt.Println("wrote", path, "features", rebuild.BuilderImageFeatureTag)
}
