package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/edwinzhancn/lumen-sdk/pkg/types"
	pb "github.com/edwinzhancn/lumen-sdk/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

type options struct {
	addr        string
	imageDir    string
	task        string
	modes       []string
	concurrency []int
	limit       int
	warmup      int
	nodePID     int
	outDir      string
	topK        int
}

type imageJob struct {
	Index int
	Path  string
}

type sample struct {
	Index        int     `json:"index"`
	Path         string  `json:"path"`
	Mode         string  `json:"mode"`
	Task         string  `json:"task"`
	Concurrency  int     `json:"concurrency"`
	Bytes        int     `json:"bytes"`
	ReadMS       float64 `json:"read_ms"`
	PreprocessMS float64 `json:"preprocess_ms"`
	InferMS      float64 `json:"infer_ms"`
	TotalMS      float64 `json:"total_ms"`
	Success      bool    `json:"success"`
	Error        string  `json:"error,omitempty"`
}

type memorySample struct {
	Timestamp       time.Time `json:"timestamp"`
	ClientRSSMB     float64   `json:"client_rss_mb"`
	NodeRSSMB       float64   `json:"node_rss_mb,omitempty"`
	ClientHeapMB    float64   `json:"client_heap_mb"`
	ClientHeapSysMB float64   `json:"client_heap_sys_mb"`
}

type summary struct {
	Task         string         `json:"task"`
	Mode         string         `json:"mode"`
	Concurrency  int            `json:"concurrency"`
	Images       int            `json:"images"`
	Success      int            `json:"success"`
	Failed       int            `json:"failed"`
	DurationMS   float64        `json:"duration_ms"`
	Throughput   float64        `json:"throughput_images_per_sec"`
	LatencyTotal latencySummary `json:"latency_total_ms"`
	LatencyRead  latencySummary `json:"latency_read_ms"`
	LatencyPre   latencySummary `json:"latency_preprocess_ms"`
	LatencyInfer latencySummary `json:"latency_infer_ms"`
	Memory       memorySummary  `json:"memory"`
	Errors       map[string]int `json:"errors,omitempty"`
}

type latencySummary struct {
	P50 float64 `json:"p50"`
	P90 float64 `json:"p90"`
	P95 float64 `json:"p95"`
	P99 float64 `json:"p99"`
	Max float64 `json:"max"`
}

type memorySummary struct {
	ClientPeakRSSMB  float64 `json:"client_peak_rss_mb"`
	NodePeakRSSMB    float64 `json:"node_peak_rss_mb,omitempty"`
	ClientPeakHeapMB float64 `json:"client_peak_heap_mb"`
}

type taskContract struct {
	ServiceName  string
	PreprocessID string
}

func main() {
	opts, err := parseOptions()
	if err != nil {
		fatal(err)
	}

	paths, err := collectImages(opts.imageDir, opts.limit)
	if err != nil {
		fatal(err)
	}
	if len(paths) == 0 {
		fatal(fmt.Errorf("no images found in %s", opts.imageDir))
	}

	ctx := context.Background()
	conn, err := grpc.NewClient(opts.addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallSendMsgSize(64*1024*1024),
			grpc.MaxCallRecvMsgSize(64*1024*1024),
		),
	)
	if err != nil {
		fatal(err)
	}
	defer conn.Close()

	client := pb.NewInferenceClient(conn)
	if _, err := client.Health(ctx, &emptypb.Empty{}); err != nil {
		fatal(fmt.Errorf("node health check failed: %w", err))
	}
	contract, err := findTaskContract(ctx, client, opts.task)
	if err != nil {
		fatal(err)
	}
	fmt.Printf("node=%s task=%s service=%s preprocess_id=%s images=%d\n", opts.addr, opts.task, contract.ServiceName, contract.PreprocessID, len(paths))

	for _, mode := range opts.modes {
		for _, concurrency := range opts.concurrency {
			if opts.warmup > 0 {
				warm := minInt(opts.warmup, len(paths))
				fmt.Printf("warmup mode=%s concurrency=%d images=%d\n", mode, concurrency, warm)
				_, _ = runScenario(ctx, client, contract, opts, paths[:warm], mode, concurrency, false)
			}

			fmt.Printf("bench mode=%s concurrency=%d images=%d\n", mode, concurrency, len(paths))
			sum, err := runScenario(ctx, client, contract, opts, paths, mode, concurrency, true)
			if err != nil {
				fatal(err)
			}
			printSummary(sum)
		}
	}
}

func parseOptions() (options, error) {
	var modeText string
	var concurrencyText string
	opts := options{}
	flag.StringVar(&opts.addr, "addr", "127.0.0.1:50051", "Lumen node gRPC address")
	flag.StringVar(&opts.imageDir, "image-dir", "", "directory containing benchmark images")
	flag.StringVar(&opts.task, "task", types.TaskSemanticImageEmbed, "task: semantic_image_embed or bioclip_classify")
	flag.StringVar(&modeText, "mode", "both", "raw, tensor, or both")
	flag.StringVar(&concurrencyText, "concurrency", "1,4,8", "comma-separated concurrency values")
	flag.IntVar(&opts.limit, "limit", 500, "maximum image count")
	flag.IntVar(&opts.warmup, "warmup", 20, "warmup image count per scenario")
	flag.IntVar(&opts.nodePID, "node-pid", 0, "optional node process PID for RSS sampling")
	flag.StringVar(&opts.outDir, "out", "", "optional output directory for JSONL and summary JSON")
	flag.IntVar(&opts.topK, "top-k", 5, "BioCLIP top_k")
	flag.Parse()

	if strings.TrimSpace(opts.imageDir) == "" {
		return opts, errors.New("--image-dir is required")
	}
	modes, err := parseModes(modeText)
	if err != nil {
		return opts, err
	}
	concurrency, err := parseConcurrency(concurrencyText)
	if err != nil {
		return opts, err
	}
	opts.modes = modes
	opts.concurrency = concurrency
	return opts, nil
}

func parseModes(value string) ([]string, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "both" {
		return []string{"raw", "tensor"}, nil
	}
	parts := strings.Split(value, ",")
	var modes []string
	for _, part := range parts {
		mode := strings.TrimSpace(part)
		switch mode {
		case "raw", "tensor":
			modes = append(modes, mode)
		default:
			return nil, fmt.Errorf("unsupported mode %q", mode)
		}
	}
	return modes, nil
}

func parseConcurrency(value string) ([]int, error) {
	parts := strings.Split(value, ",")
	var out []int
	for _, part := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("invalid concurrency %q", part)
		}
		out = append(out, n)
	}
	return out, nil
}

func collectImages(dir string, limit int) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".jpg", ".jpeg", ".png", ".webp":
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	if limit > 0 && len(paths) > limit {
		paths = paths[:limit]
	}
	return paths, nil
}

func findTaskContract(ctx context.Context, client pb.InferenceClient, taskName string) (taskContract, error) {
	stream, err := client.StreamCapabilities(ctx, &emptypb.Empty{})
	if err != nil {
		return taskContract{}, err
	}
	for {
		capability, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return taskContract{}, err
		}
		for _, task := range capability.GetTasks() {
			if task.GetName() == taskName {
				return taskContract{ServiceName: capability.GetServiceName(), PreprocessID: task.GetTensorPreprocessId()}, nil
			}
		}
	}
	return taskContract{}, fmt.Errorf("task %q not advertised by node", taskName)
}

func runScenario(ctx context.Context, client pb.InferenceClient, contract taskContract, opts options, paths []string, mode string, concurrency int, record bool) (summary, error) {
	start := time.Now()
	jobs := make(chan imageJob)
	results := make(chan sample, len(paths))
	var counter atomic.Int64

	var memStop chan struct{}
	var memDone chan []memorySample
	if record {
		memStop = make(chan struct{})
		memDone = make(chan []memorySample, 1)
		go monitorMemory(memStop, memDone, opts.nodePID)
	}

	var wg sync.WaitGroup
	for worker := 0; worker < concurrency; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				correlationID := fmt.Sprintf("bench-%s-%d", mode, counter.Add(1))
				results <- runOne(ctx, client, contract, opts, job, mode, concurrency, correlationID)
			}
		}()
	}

	for index, path := range paths {
		jobs <- imageJob{Index: index, Path: path}
	}
	close(jobs)
	wg.Wait()
	close(results)

	var memory []memorySample
	if record {
		close(memStop)
		memory = <-memDone
	}

	var samples []sample
	for result := range results {
		samples = append(samples, result)
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i].Index < samples[j].Index })
	sum := summarize(opts.task, mode, concurrency, samples, memory, time.Since(start))
	if record && opts.outDir != "" {
		if err := writeResults(opts.outDir, sum, samples, memory); err != nil {
			return sum, err
		}
	}
	return sum, nil
}

func runOne(ctx context.Context, client pb.InferenceClient, contract taskContract, opts options, job imageJob, mode string, concurrency int, correlationID string) sample {
	t0 := time.Now()
	payload, err := os.ReadFile(job.Path)
	readMS := elapsedMS(t0)
	result := sample{Index: job.Index, Path: job.Path, Mode: mode, Task: opts.task, Concurrency: concurrency, Bytes: len(payload), ReadMS: readMS}
	if err != nil {
		result.Error = err.Error()
		return result
	}

	var req *pb.InferRequest
	preprocessStart := time.Now()
	if mode == "tensor" {
		registry := types.DefaultTensorPreprocessorRegistry()
		preprocessor, ok := registry.Lookup(contract.PreprocessID)
		if !ok {
			result.Error = fmt.Sprintf("unknown preprocess id %q", contract.PreprocessID)
			return result
		}
		tensor, err := preprocessor.Preprocess(ctx, types.ImageInput{Encoded: payload, PayloadMIME: mimeForPath(job.Path)})
		result.PreprocessMS = elapsedMS(preprocessStart)
		if err != nil {
			result.Error = err.Error()
			return result
		}
		req = types.NewInferRequest(opts.task).
			WithCorrelationID(correlationID).
			ForTensorInput(tensor.Payload, tensor.PayloadMIME, tensor.Descriptor).
			WithService(contract.ServiceName).
			Build()
	} else {
		result.PreprocessMS = 0
		req = rawRequest(opts, contract, correlationID, payload, mimeForPath(job.Path))
	}

	inferStart := time.Now()
	_, err = infer(ctx, client, req)
	result.InferMS = elapsedMS(inferStart)
	result.TotalMS = result.ReadMS + result.PreprocessMS + result.InferMS
	if err != nil {
		result.Error = err.Error()
		return result
	}
	result.Success = true
	return result
}

func rawRequest(opts options, contract taskContract, correlationID string, payload []byte, mime string) *pb.InferRequest {
	builder := types.NewInferRequest(opts.task).WithCorrelationID(correlationID).WithService(contract.ServiceName)
	switch opts.task {
	case types.TaskSemanticImageEmbed:
		return builder.ForSemanticImageEmbed(payload, mime).Build()
	case types.TaskBioCLIPClassify:
		return builder.ForBioCLIPClassify(payload, mime, opts.topK).Build()
	default:
		return builder.WithPayload(payload, mime).Build()
	}
}

func infer(ctx context.Context, client pb.InferenceClient, req *pb.InferRequest) (*pb.InferResponse, error) {
	stream, err := client.Infer(ctx)
	if err != nil {
		return nil, err
	}
	if err := stream.Send(req); err != nil {
		return nil, err
	}
	if err := stream.CloseSend(); err != nil {
		return nil, err
	}
	var final *pb.InferResponse
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if resp.GetError() != nil && resp.GetError().GetMessage() != "" {
			return nil, errors.New(resp.GetError().GetMessage())
		}
		final = resp
		if resp.GetIsFinal() {
			break
		}
	}
	if final == nil {
		return nil, errors.New("empty Infer response")
	}
	return final, nil
}

func mimeForPath(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}

func summarize(task, mode string, concurrency int, samples []sample, memory []memorySample, duration time.Duration) summary {
	var total, read, pre, inferTimes []float64
	errorsByText := map[string]int{}
	success := 0
	for _, s := range samples {
		if s.Success {
			success++
		}
		if s.Error != "" {
			errorsByText[s.Error]++
		}
		total = append(total, s.TotalMS)
		read = append(read, s.ReadMS)
		pre = append(pre, s.PreprocessMS)
		inferTimes = append(inferTimes, s.InferMS)
	}
	mem := memorySummary{}
	for _, m := range memory {
		mem.ClientPeakRSSMB = math.Max(mem.ClientPeakRSSMB, m.ClientRSSMB)
		mem.NodePeakRSSMB = math.Max(mem.NodePeakRSSMB, m.NodeRSSMB)
		mem.ClientPeakHeapMB = math.Max(mem.ClientPeakHeapMB, m.ClientHeapMB)
	}
	if len(errorsByText) == 0 {
		errorsByText = nil
	}
	seconds := duration.Seconds()
	throughput := 0.0
	if seconds > 0 {
		throughput = float64(success) / seconds
	}
	return summary{Task: task, Mode: mode, Concurrency: concurrency, Images: len(samples), Success: success, Failed: len(samples) - success, DurationMS: float64(duration.Microseconds()) / 1000.0, Throughput: throughput, LatencyTotal: summarizeLatency(total), LatencyRead: summarizeLatency(read), LatencyPre: summarizeLatency(pre), LatencyInfer: summarizeLatency(inferTimes), Memory: mem, Errors: errorsByText}
}

func summarizeLatency(values []float64) latencySummary {
	if len(values) == 0 {
		return latencySummary{}
	}
	sort.Float64s(values)
	return latencySummary{P50: percentile(values, 0.50), P90: percentile(values, 0.90), P95: percentile(values, 0.95), P99: percentile(values, 0.99), Max: values[len(values)-1]}
}

func percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	idx := int(math.Ceil(p*float64(len(values)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(values) {
		idx = len(values) - 1
	}
	return values[idx]
}

func writeResults(outDir string, sum summary, samples []sample, memory []memorySample) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	base := fmt.Sprintf("%s_%s_c%d", sum.Task, sum.Mode, sum.Concurrency)
	jsonlPath := filepath.Join(outDir, base+".jsonl")
	jsonl, err := os.Create(jsonlPath)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(jsonl)
	for _, sample := range samples {
		if err := enc.Encode(sample); err != nil {
			_ = jsonl.Close()
			return err
		}
	}
	if err := jsonl.Close(); err != nil {
		return err
	}

	summaryPath := filepath.Join(outDir, base+".summary.json")
	if err := writeJSON(summaryPath, sum); err != nil {
		return err
	}
	if len(memory) > 0 {
		memPath := filepath.Join(outDir, base+".memory.jsonl")
		memFile, err := os.Create(memPath)
		if err != nil {
			return err
		}
		memEnc := json.NewEncoder(memFile)
		for _, sample := range memory {
			if err := memEnc.Encode(sample); err != nil {
				_ = memFile.Close()
				return err
			}
		}
		if err := memFile.Close(); err != nil {
			return err
		}
	}
	return nil
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func monitorMemory(stop <-chan struct{}, done chan<- []memorySample, nodePID int) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	samples := []memorySample{readMemory(nodePID)}
	for {
		select {
		case <-stop:
			samples = append(samples, readMemory(nodePID))
			done <- samples
			return
		case <-ticker.C:
			samples = append(samples, readMemory(nodePID))
		}
	}
}

func readMemory(nodePID int) memorySample {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	return memorySample{Timestamp: time.Now(), ClientRSSMB: rssMB(os.Getpid()), NodeRSSMB: rssMB(nodePID), ClientHeapMB: bytesToMB(stats.HeapAlloc), ClientHeapSysMB: bytesToMB(stats.HeapSys)}
}

func rssMB(pid int) float64 {
	if pid <= 0 {
		return 0
	}
	cmd := exec.Command("ps", "-o", "rss=", "-p", strconv.Itoa(pid))
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	kb, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		return 0
	}
	return kb / 1024.0
}

func bytesToMB(v uint64) float64        { return float64(v) / 1024.0 / 1024.0 }
func elapsedMS(start time.Time) float64 { return float64(time.Since(start).Microseconds()) / 1000.0 }
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func printSummary(sum summary) {
	fmt.Printf("%s/%s c=%d images=%d ok=%d fail=%d throughput=%.2f img/s total_p50=%.1fms total_p95=%.1fms infer_p95=%.1fms pre_p95=%.1fms client_rss_peak=%.1fMB node_rss_peak=%.1fMB\n",
		sum.Task, sum.Mode, sum.Concurrency, sum.Images, sum.Success, sum.Failed, sum.Throughput, sum.LatencyTotal.P50, sum.LatencyTotal.P95, sum.LatencyInfer.P95, sum.LatencyPre.P95, sum.Memory.ClientPeakRSSMB, sum.Memory.NodePeakRSSMB)
	if len(sum.Errors) > 0 {
		fmt.Printf("errors: %+v\n", sum.Errors)
	}
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}
