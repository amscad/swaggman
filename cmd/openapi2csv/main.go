package main

import (
	"fmt"
	"log"
	"regexp"

	"github.com/grokify/simplego/io/ioutilmore"
	csv "github.com/grokify/spectrum/openapi2/openapi2csv"
	"github.com/jessevdk/go-flags"
)

type Options struct {
	Directory string `short:"d" long:"dir" description:"Source Directory" required:"true"`
	Regexp    string `short:"r" long:"regexp" description:"matching " required:"true"`
	Output    string `short:"o" long:"output" description:"Output CSV File" required:"true"`
}

func main() {
	var opts Options
	_, err := flags.Parse(&opts)
	if err != nil {
		log.Fatal(err)
	}

	_, filepaths, err := ioutilmore.ReadDirMore(
		opts.Directory, regexp.MustCompile(opts.Regexp), true, true)
	if err != nil {
		log.Fatal(err)
	}
	tbl, err := csv.TableFromSpecFiles(filepaths, true)
	if err != nil {
		log.Fatal(fmt.Sprintf("TableFromSpecFiles [%v]\n", err.Error()))
	}

	err = tbl.WriteCSV(opts.Output)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("WROTE [%v]\n", opts.Output)

	fmt.Println("DONE")
}
