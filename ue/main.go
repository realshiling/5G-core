package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	amfpb "github.com/5g-core/proto/amf"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type options struct {
	address, action, ueID, scenario, output string
	count, concurrency                      int
	timeout                                 time.Duration
	seed                                    int64
}
type result struct {
	UEID         string
	Success      bool
	Latency      time.Duration
	Signal, SINR float32
	Message      string
}

func parseFlags() options {
	var o options
	flag.StringVar(&o.address, "address", "localhost:50051", "AMF gRPC地址")
	flag.StringVar(&o.action, "action", "register", "register、deregister或benchmark")
	flag.StringVar(&o.ueID, "id", "UE-001", "单UE ID")
	flag.StringVar(&o.scenario, "scenario", "stable", "stable、edge、degrading或mixed")
	flag.StringVar(&o.output, "output", "", "压测明细CSV路径（可选）")
	flag.IntVar(&o.count, "count", 100, "压测UE数量")
	flag.IntVar(&o.concurrency, "concurrency", 20, "压测并发数")
	flag.DurationVar(&o.timeout, "timeout", 5*time.Second, "单请求超时")
	flag.Int64Var(&o.seed, "seed", 42, "信道随机种子")
	flag.Parse()
	return o
}

func main() {
	o := parseFlags()
	conn, err := grpc.NewClient(o.address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("连接 AMF: %v", err)
	}
	defer conn.Close()
	client := amfpb.NewAMFServiceClient(conn)
	switch o.action {
	case "register":
		r := register(client, o, o.ueID, sampleChannel(o.scenario, rand.New(rand.NewSource(o.seed))))
		if !r.Success {
			log.Fatalf("注册失败 ue_id=%s message=%s latency=%s", r.UEID, r.Message, r.Latency)
		}
		log.Printf("注册成功 ue_id=%s latency=%s message=%s", r.UEID, r.Latency, r.Message)
	case "deregister":
		ctx, cancel := context.WithTimeout(context.Background(), o.timeout)
		defer cancel()
		resp, err := client.Deregister(ctx, &amfpb.DeregisterRequest{UeId: o.ueID})
		if err != nil {
			log.Fatalf("注销失败: %v", err)
		}
		log.Printf("注销结果 success=%v message=%s", resp.Success, resp.Message)
	case "benchmark":
		runBenchmark(client, o)
	default:
		log.Fatalf("不支持的action: %s", o.action)
	}
}

func register(client amfpb.AMFServiceClient, o options, ueID string, channel ChannelSample) result {
	ctx, cancel := context.WithTimeout(context.Background(), o.timeout)
	defer cancel()
	started := time.Now()
	resp, err := client.Register(ctx, &amfpb.RegisterRequest{UeId: ueID, UeType: "simulator", SignalPower: channel.SignalPower, Sinr: channel.SINR})
	r := result{UEID: ueID, Latency: time.Since(started), Signal: channel.SignalPower, SINR: channel.SINR}
	if err != nil {
		r.Message = err.Error()
		return r
	}
	r.Success, r.Message = resp.Success, resp.Message
	return r
}

func runBenchmark(client amfpb.AMFServiceClient, o options) {
	if o.count < 1 || o.concurrency < 1 {
		log.Fatal("count和concurrency必须大于0")
	}
	started := time.Now()
	results := make([]result, o.count)
	sem := make(chan struct{}, o.concurrency)
	var wg sync.WaitGroup
	var completed atomic.Int64
	for i := 0; i < o.count; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(index int) {
			defer wg.Done()
			defer func() { <-sem }()
			rng := rand.New(rand.NewSource(o.seed + int64(index)))
			channel := sampleChannel(o.scenario, rng)
			results[index] = register(client, o, fmt.Sprintf("UE-%06d", index+1), channel)
			completed.Add(1)
		}(i)
	}
	wg.Wait()
	wall := time.Since(started)
	report(results, wall)
	if o.output != "" {
		if err := writeCSV(o.output, results); err != nil {
			log.Fatalf("写CSV: %v", err)
		}
		log.Printf("明细已写入 %s", o.output)
	}
}

func report(results []result, wall time.Duration) {
	latencies := make([]time.Duration, 0, len(results))
	success := 0
	for _, r := range results {
		latencies = append(latencies, r.Latency)
		if r.Success {
			success++
		}
	}
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	var total time.Duration
	for _, value := range latencies {
		total += value
	}
	percentile := func(p float64) time.Duration { index := int(float64(len(latencies)-1) * p); return latencies[index] }
	fmt.Printf("\nLiteCore 压测结果\n总请求: %d\n成功: %d\n失败: %d\n成功率: %.2f%%\n总耗时: %s\n吞吐量: %.2f req/s\n平均延迟: %s\nP50: %s\nP95: %s\nP99: %s\n最大延迟: %s\n", len(results), success, len(results)-success, float64(success)*100/float64(len(results)), wall, float64(len(results))/wall.Seconds(), total/time.Duration(len(results)), percentile(.50), percentile(.95), percentile(.99), latencies[len(latencies)-1])
}

func writeCSV(path string, results []result) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	w := csv.NewWriter(file)
	defer w.Flush()
	if err := w.Write([]string{"ue_id", "success", "latency_ms", "signal_power_dbm", "sinr_db", "message"}); err != nil {
		return err
	}
	for _, r := range results {
		if err := w.Write([]string{r.UEID, strconv.FormatBool(r.Success), fmt.Sprintf("%.3f", float64(r.Latency.Microseconds())/1000), fmt.Sprintf("%.2f", r.Signal), fmt.Sprintf("%.2f", r.SINR), r.Message}); err != nil {
			return err
		}
	}
	return w.Error()
}
