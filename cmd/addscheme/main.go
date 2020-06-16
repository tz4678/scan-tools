// Утилита для определения протокола хоста
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"scan-tools/internal/utils"
	"sync"
	"time"
)

var schemes = map[string]string{
	"https": "443",
	"http":  "80",
}

type cmdFlags struct {
	Input       string
	Output      string
	Concurrency int
	Timeout     int
	ShowHelp    bool
}

func setupFlags(flags *cmdFlags) {
	flag.StringVar(&flags.Input, "i", "-", "Input filename")
	flag.StringVar(&flags.Output, "o", "-", "Output filename")
	flag.IntVar(&flags.Concurrency, "c", 10, "Concurrent requests")
	flag.IntVar(&flags.Timeout, "t", 3, "Connect timeout in seconds")
	flag.BoolVar(&flags.ShowHelp, "h", false, "Show help and exit")
}

func main() {
	flags := cmdFlags{}
	setupFlags(&flags)
	flag.Parse()
	run(&flags)
}

func run(flags *cmdFlags) {
	if flags.ShowHelp {
		fmt.Fprintf(os.Stderr, "Usage %s [options...]\n\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(1)
	}
	in := os.Stdin
	if flags.Input != "-" {
		path, err := utils.ExpandPath(flags.Input)
		if err != nil {
			log.Panic(err)
		}
		file, err := os.Open(path)
		if err != nil {
			log.Panic(err)
		}
		defer file.Close()
		in = file
	}
	out := os.Stdout
	if flags.Output != "-" {
		path, err := utils.ExpandPath(flags.Output)
		if err != nil {
			log.Panic(err)
		}
		file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
		if err != nil {
			log.Panic(err)
		}
		defer file.Close()
		out = file
	}
	hosts, err := utils.ReadLines(in)
	if err != nil {
		log.Panic(err)
	}
	// fmt.Printf("%#v\n", hosts)
	hostsLen := len(hosts)
	numberOfWorkers := utils.Min(flags.Concurrency, hostsLen)
	jobs := make(chan string, numberOfWorkers)
	go func() {
		for _, host := range hosts {
			// skip empty lines
			if host != "" {
				jobs <- host
			}
		}
		close(jobs)
	}()
	results := make(chan string)
	var wg sync.WaitGroup
	wg.Add(hostsLen)
	timeout := time.Duration(flags.Timeout) * time.Second
	for i := 0; i < numberOfWorkers; i++ {
		go worker(jobs, results, timeout, &wg)
	}
	// из-за того что wg.Wait() не завернул в горутину работало неправильно
	go func() {
		wg.Wait()
		close(results)
	}()
	for result := range results {
		fmt.Fprintln(out, result)
	}
}

func worker(
	jobs <-chan string,
	results chan<- string,
	timeout time.Duration,
	wg *sync.WaitGroup,
) {
	for host := range jobs {
		for scheme, port := range schemes {
			conn, _ := net.DialTimeout(
				"tcp",
				net.JoinHostPort(host, port),
				timeout,
			)
			if conn != nil {
				conn.Close()
				hostname := fmt.Sprintf("%s://%s", scheme, host)
				results <- hostname
				break
			}
		}
		wg.Done()
	}
}
