package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
)

var (
	namenodeJmxURL     = flag.String("namenode.jmx.url", "http://localhost:50070/jmx", "Namenode JMX URL.")
	namenodeJmxTimeout = flag.Duration("namenode.jmx.timeout", 5*time.Second, "Timeout reading from namenode JMX URL.")
	pidFile            = flag.String("namenode.pid-file", "", "Optional path to a file containing the namenode PID for additional metrics.")
	showVersion        = flag.Bool("version", false, "Print version information.")
	listenAddress      = flag.String("web.listen-address", ":9779", "Address to listen on for web interface and telemetry.")
	metricsPath        = flag.String("web.telemetry-path", "/metrics", "Path under which to expose metrics.")
)

const (
	namespace = "namenode"
)

// Exporter collects metrics from a namenode server.
type Exporter struct {
	url        string
	httpClient *http.Client

	// namenode server health metrics
	up            *prometheus.Desc // DONE!!! gauge -> validated by connecting the the JMX endpoint
	uptime        *prometheus.Desc // DONE!!! gauge -> "java.lang:type=Runtime" -> Uptime
	state         *prometheus.Desc // DONE!!! gauge -> "Hadoop:service=NameNode,name=NameNodeStatus" -> State -> string
	fsOperational *prometheus.Desc // DONE!!! gauge -> "Hadoop:service=NameNode,name=FSNamesystemState" -> FSState -> string
	safemodeOn    *prometheus.Desc // DONE!!! gauge -> "Hadoop:service=NameNode,name=NameNodeInfo" -> Safemode
	dataNodesLive *prometheus.Desc // DONE!!! gauge -> "Hadoop:service=NameNode,name=FSNamesystemState" -> NumLiveDataNodes -> int
	dataNodesDead *prometheus.Desc // DONE!!! gauge -> "Hadoop:service=NameNode,name=FSNamesystemState" -> NumDeadDataNodes -> int

	// dfs capacity metrics
	dfsFilesTotal             *prometheus.Desc // DONE!!! gauge -> "Hadoop:service=NameNode,name=FSNamesystemState" -> FilesTotal
	dfsPercentUsed            *prometheus.Desc // DONE!!! gauge -> "Hadoop:service=NameNode,name=NameNodeInfo" -> PercentUsed
	dfsPercentRemaining       *prometheus.Desc // DONE!!! gauge -> "Hadoop:service=NameNode,name=NameNodeInfo" -> PercentRemaining
	dfsCapacityBytesTotal     *prometheus.Desc // DONE!!! gauge -> "Hadoop:service=NameNode,name=FSNamesystemState" -> CapacityTotal -> int
	dfsCapacityBytesUsed      *prometheus.Desc // DONE!!! gauge -> "Hadoop:service=NameNode,name=FSNamesystemState" -> CapacityUsed -> int
	dfsCapacityBytesRemaining *prometheus.Desc // DONE!!! gauge -> "Hadoop:service=NameNode,name=FSNamesystemState" -> CapacityRemaining -> int
	dfsNonDfsBytesUsed        *prometheus.Desc // DONE!!! gauge -> "Hadoop:service=NameNode,name=NameNodeInfo" -> NonDfsUsedSpace -> int

	// dfs block metrics
	dfsBlocksTotal                  *prometheus.Desc // gauge -> "Hadoop:service=NameNode,name=FSNamesystem" -> BlocksTotal
	dfsBlocksUnderReplicated        *prometheus.Desc // gauge -> "Hadoop:service=NameNode,name=FSNamesystem" -> UnderReplicatedBlocks
	dfsBlocksPendingReplication     *prometheus.Desc // gauge -> "Hadoop:service=NameNode,name=FSNamesystem" -> PendingReplicationBlocks
	dfsBlocksScheduledReplication   *prometheus.Desc // gauge -> "Hadoop:service=NameNode,name=FSNamesystem" -> ScheduledReplicationBlocks
	dfsBlocksPostponedMisreplicated *prometheus.Desc // gauge -> "Hadoop:service=NameNode,name=FSNamesystem" -> PostponedMisreplicatedBlocks
	dfsBlocksPendingDeletion        *prometheus.Desc // gauge -> "Hadoop:service=NameNode,name=FSNamesystem" -> PendingDeletionBlocks
	dfsBlocksMissing                *prometheus.Desc // gauge -> "Hadoop:service=NameNode,name=FSNamesystem" -> MissingBlocks
	dfsBlocksCorrupt                *prometheus.Desc // gauge -> "Hadoop:service=NameNode,name=FSNamesystem" -> CorruptBlocks
	dfsBlocksExcess                 *prometheus.Desc // gauge -> "Hadoop:service=NameNode,name=FSNamesystem" -> ExcessBlocks
	dfsBlockPoolBytesUsed           *prometheus.Desc // DONE!!! gauge -> "Hadoop:service=NameNode,name=NameNodeInfo" -> BlockPoolUsedSpace
	dfsBlockPoolPercentUsed         *prometheus.Desc // DONE!!! gauge -> "Hadoop:service=NameNode,name=NameNodeInfo" -> PercentBlockPoolUsed

	// namenode jvm metrics
	jvmLogFatal                     *prometheus.Desc // DONE!!! counter -> "Hadoop:service=NameNode,name=JvmMetrics" -> LogFatal
	jvmLogError                     *prometheus.Desc // DONE!!! counter -> "Hadoop:service=NameNode,name=JvmMetrics" -> LogError
	jvmLogWarn                      *prometheus.Desc // DONE!!! counter -> "Hadoop:service=NameNode,name=JvmMetrics" -> LogWarn
	jvmLogInfo                      *prometheus.Desc // DONE!!! counter -> "Hadoop:service=NameNode,name=JvmMetrics" -> LogInfo
	jvmMemHeapMegabytesUsed         *prometheus.Desc // DONE!!! gauge -> "Hadoop:service=NameNode,name=JvmMetrics" -> MemHeapUsedM
	jvmMemHeapMegabytesCommitted    *prometheus.Desc // DONE!!! gauge -> "Hadoop:service=NameNode,name=JvmMetrics" -> MemHeapCommittedM
	jvmMemNonHeapMegabytesUsed      *prometheus.Desc // DONE!!! gauge -> "Hadoop:service=NameNode,name=JvmMetrics" -> MemNonHeapUsedM
	jvmMemNonHeapMegabytesCommitted *prometheus.Desc // DONE!!! gauge -> "Hadoop:service=NameNode,name=JvmMetrics" -> MemNonHeapCommittedM
	jvmThreadsNew                   *prometheus.Desc // DONE!!! gauge -> "Hadoop:service=NameNode,name=JvmMetrics" -> ThreadsNew
	jvmThreadsRunnable              *prometheus.Desc // DONE!!! gauge -> "Hadoop:service=NameNode,name=JvmMetrics" -> ThreadsRunnable
	jvmThreadsBlocked               *prometheus.Desc // DONE!!! gauge -> "Hadoop:service=NameNode,name=JvmMetrics" -> ThreadsBlocked
	jvmThreadsWaiting               *prometheus.Desc // DONE!!! gauge -> "Hadoop:service=NameNode,name=JvmMetrics" -> ThreadsWaiting
	jvmThreadsTimedWaiting          *prometheus.Desc // DONE!!! gauge -> "Hadoop:service=NameNode,name=JvmMetrics" -> ThreadsTimedWaiting
	jvmThreadsTerminated            *prometheus.Desc // DONE!!! gauge -> "Hadoop:service=NameNode,name=JvmMetrics" -> ThreadsTerminated
}

// NewExporter returns an initialized exporter.
func NewExporter(url string, timeout time.Duration) *Exporter {
	return &Exporter{
		url:        url,
		httpClient: &http.Client{Timeout: timeout},

		// namenode server health metrics
		up: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "up"),
			"Could the namenode be reached.",
			nil,
			nil,
		),
		uptime: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "uptime_seconds"),
			"Number of seconds since the namenode started.",
			nil,
			nil,
		),
		state: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "state"),
			"Indicate namenode state (0 - standby, 1 - active).",
			nil,
			nil,
		),
		fsOperational: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "fs_operational"),
			"The filesystem state of this namenode.",
			nil,
			nil,
		),
		safemodeOn: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "safemode_on"),
			"The safemode state of this namenode.",
			nil,
			nil,
		),
		dataNodesLive: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "data_nodes_live"),
			"The number of live datanodes in this DFS.",
			nil,
			nil,
		),
		dataNodesDead: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "data_nodes_dead"),
			"The number of dead datanodes in this DFS.",
			nil,
			nil,
		),

		// dfs capacity metrics
		dfsFilesTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "dfs", "files_total"),
			"Total number of files in DFS.",
			nil,
			nil,
		),
		dfsPercentUsed: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "dfs", "percent_used"),
			"TODO(fahlke): describe this metric",
			nil,
			nil,
		),
		dfsPercentRemaining: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "dfs", "percent_remaining"),
			"TODO(fahlke): describe this metric",
			nil,
			nil,
		),
		dfsCapacityBytesTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "dfs", "capacity_bytes_total"),
			"Total configured DFS storage capacity in bytes.",
			nil,
			nil,
		),
		dfsCapacityBytesUsed: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "dfs", "capacity_bytes_used"),
			"The usage of the DFS in bytes.",
			nil,
			nil,
		),
		dfsCapacityBytesRemaining: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "dfs", "capacity_bytes_remaining"),
			"The remaining capacity of the DFS in bytes.",
			nil,
			nil,
		),
		dfsNonDfsBytesUsed: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "dfs", "non_dfs_bytes_used"),
			"Non-DFS usage in bytes.",
			nil,
			nil,
		),

		// dfs block metrics
		dfsBlocksTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "dfs", "blocks_total"),
			"Total blocks in DFS.",
			nil,
			nil,
		),
		dfsBlocksUnderReplicated: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "dfs", "blocks_under_replicated"),
			"Under replicated blocks in DFS.",
			nil,
			nil,
		),
		dfsBlocksPendingReplication: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "dfs", "blocks_pending_replication"),
			"TODO(fahlke): describe this metric",
			nil,
			nil,
		),
		dfsBlocksScheduledReplication: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "dfs", "blocks_scheduled_replication"),
			"TODO(fahlke): describe this metric",
			nil,
			nil,
		),
		dfsBlocksPostponedMisreplicated: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "dfs", "blocks_postponed_misreplicated"),
			"TODO(fahlke): describe this metric",
			nil,
			nil,
		),
		dfsBlocksPendingDeletion: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "dfs", "blocks_pending_deletion"),
			"TODO(fahlke): describe this metric",
			nil,
			nil,
		),
		dfsBlocksMissing: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "dfs", "blocks_missing"),
			"Missing blocks in DFS.",
			nil,
			nil,
		),
		dfsBlocksCorrupt: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "dfs", "blocks_corrupt"),
			"Corrupted blocks in DFS.",
			nil,
			nil,
		),
		dfsBlocksExcess: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "dfs", "blocks_excess"),
			"Excess blocks in DFS.",
			nil,
			nil,
		),
		dfsBlockPoolBytesUsed: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "dfs", "block_pool_bytes_used"),
			"TODO(fahlke): describe this metric",
			nil,
			nil,
		),
		dfsBlockPoolPercentUsed: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "dfs", "block_pool_percent_used"),
			"TODO(fahlke): describe this metric",
			nil,
			nil,
		),

		// namenode jvm metrics
		jvmLogFatal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "jvm", "log_fatal"),
			"TODO(fahlke): describe this metric",
			nil,
			nil,
		),
		jvmLogError: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "jvm", "log_error"),
			"TODO(fahlke): describe this metric",
			nil,
			nil,
		),
		jvmLogWarn: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "jvm", "log_warn"),
			"TODO(fahlke): describe this metric",
			nil,
			nil,
		),
		jvmLogInfo: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "jvm", "log_info"),
			"TODO(fahlke): describe this metric",
			nil,
			nil,
		),
		jvmMemHeapMegabytesUsed: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "jvm", "mem_heap_megabytes_used"),
			"TODO(fahlke): describe this metric",
			nil,
			nil,
		),
		jvmMemHeapMegabytesCommitted: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "jvm", "mem_heap_megabytes_committed"),
			"TODO(fahlke): describe this metric",
			nil,
			nil,
		),
		jvmMemNonHeapMegabytesUsed: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "jvm", "mem_non_heap_megabytes_used"),
			"TODO(fahlke): describe this metric",
			nil,
			nil,
		),
		jvmMemNonHeapMegabytesCommitted: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "jvm", "mem_non_heap_megabytes_committed"),
			"TODO(fahlke): describe this metric",
			nil,
			nil,
		),
		jvmThreadsNew: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "jvm", "threads_new"),
			"TODO(fahlke): describe this metric",
			nil,
			nil,
		),
		jvmThreadsRunnable: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "jvm", "threads_runnable"),
			"TODO(fahlke): describe this metric",
			nil,
			nil,
		),
		jvmThreadsBlocked: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "jvm", "threads_blocked"),
			"TODO(fahlke): describe this metric",
			nil,
			nil,
		),
		jvmThreadsWaiting: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "jvm", "threads_waiting"),
			"TODO(fahlke): describe this metric",
			nil,
			nil,
		),
		jvmThreadsTimedWaiting: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "jvm", "threads_timed_waiting"),
			"TODO(fahlke): describe this metric",
			nil,
			nil,
		),
		jvmThreadsTerminated: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "jvm", "threads_terminated"),
			"TODO(fahlke): describe this metric",
			nil,
			nil,
		),
	}
}

// Describe describes all the metrics exported by the namenode exporter.
// It implements prometheus.Collector.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	// namenode server health metrics
	ch <- e.up
	ch <- e.uptime
	ch <- e.state
	ch <- e.fsOperational
	ch <- e.safemodeOn
	ch <- e.dataNodesLive
	ch <- e.dataNodesDead

	// dfs capacity metrics
	ch <- e.dfsFilesTotal
	ch <- e.dfsPercentUsed
	ch <- e.dfsPercentRemaining
	ch <- e.dfsCapacityBytesTotal
	ch <- e.dfsCapacityBytesUsed
	ch <- e.dfsCapacityBytesRemaining
	ch <- e.dfsNonDfsBytesUsed

	// dfs block metrics
	ch <- e.dfsBlocksTotal
	ch <- e.dfsBlocksUnderReplicated
	ch <- e.dfsBlocksPendingReplication
	ch <- e.dfsBlocksScheduledReplication
	ch <- e.dfsBlocksPostponedMisreplicated
	ch <- e.dfsBlocksPendingDeletion
	ch <- e.dfsBlocksMissing
	ch <- e.dfsBlocksCorrupt
	ch <- e.dfsBlocksExcess
	ch <- e.dfsBlockPoolBytesUsed
	ch <- e.dfsBlockPoolPercentUsed

	// namenode jvm metrics
	ch <- e.jvmLogFatal
	ch <- e.jvmLogError
	ch <- e.jvmLogWarn
	ch <- e.jvmLogInfo
	ch <- e.jvmMemHeapMegabytesUsed
	ch <- e.jvmMemHeapMegabytesCommitted
	ch <- e.jvmMemNonHeapMegabytesUsed
	ch <- e.jvmMemNonHeapMegabytesCommitted
	ch <- e.jvmThreadsNew
	ch <- e.jvmThreadsRunnable
	ch <- e.jvmThreadsBlocked
	ch <- e.jvmThreadsWaiting
	ch <- e.jvmThreadsTimedWaiting
	ch <- e.jvmThreadsTerminated
}

type jmxEnvelope struct {
	Beans []jmxBean `json:"beans"`
}

type jmxBean map[string]interface{}

func mustNewConstBoolMetric(desc *prometheus.Desc, valueType prometheus.ValueType, value bool, labelValues ...string) prometheus.Metric {
	var fval float64
	if value {
		fval = 1
	}

	return prometheus.MustNewConstMetric(desc, valueType, fval, labelValues...)
}

// Collect fetches the statistics from the configured Namenode server, and
// delivers them as Prometheus metrics. It implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	resp, err := e.httpClient.Get(e.url)
	if err != nil {
		ch <- prometheus.MustNewConstMetric(e.up, prometheus.GaugeValue, 0)
		log.Errorf("Failed to collect metrics from namenode: %s", err)
		return
	}
	defer func() {
		ioutil.ReadAll(resp.Body) // Mindless drain body upon exit
		resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		ch <- prometheus.MustNewConstMetric(e.up, prometheus.GaugeValue, 0)
		log.Errorf("Failed to collect metrics from namenode: HTTP status code %d", resp.StatusCode)
		return
	}

	var envelope jmxEnvelope
	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(&envelope)
	if err != nil {
		ch <- prometheus.MustNewConstMetric(e.up, prometheus.GaugeValue, 0)
		log.Errorf("Failed to collect metrics from namenode: %s", err)
		return
	}
	ch <- prometheus.MustNewConstMetric(e.up, prometheus.GaugeValue, 1)

	for _, nameDataMap := range envelope.Beans {
		switch nameDataMap["name"] {
		case "java.lang:type=Runtime":
			ch <- prometheus.MustNewConstMetric(e.uptime, prometheus.GaugeValue, nameDataMap["Uptime"].(float64))
		case "Hadoop:service=NameNode,name=NameNodeStatus":
			ch <- mustNewConstBoolMetric(e.state, prometheus.GaugeValue, nameDataMap["State"] == "active")
		case "Hadoop:service=NameNode,name=FSNamesystem":
			ch <- prometheus.MustNewConstMetric(e.dfsBlocksTotal, prometheus.GaugeValue, nameDataMap["BlocksTotal"].(float64))
			ch <- prometheus.MustNewConstMetric(e.dfsBlocksUnderReplicated, prometheus.GaugeValue, nameDataMap["UnderReplicatedBlocks"].(float64))
			ch <- prometheus.MustNewConstMetric(e.dfsBlocksPendingReplication, prometheus.GaugeValue, nameDataMap["PendingReplicationBlocks"].(float64))
			ch <- prometheus.MustNewConstMetric(e.dfsBlocksScheduledReplication, prometheus.GaugeValue, nameDataMap["ScheduledReplicationBlocks"].(float64))
			ch <- prometheus.MustNewConstMetric(e.dfsBlocksPostponedMisreplicated, prometheus.GaugeValue, nameDataMap["PostponedMisreplicatedBlocks"].(float64))
			ch <- prometheus.MustNewConstMetric(e.dfsBlocksPendingDeletion, prometheus.GaugeValue, nameDataMap["PendingDeletionBlocks"].(float64))
			ch <- prometheus.MustNewConstMetric(e.dfsBlocksMissing, prometheus.GaugeValue, nameDataMap["MissingBlocks"].(float64))
			ch <- prometheus.MustNewConstMetric(e.dfsBlocksCorrupt, prometheus.GaugeValue, nameDataMap["CorruptBlocks"].(float64))
			ch <- prometheus.MustNewConstMetric(e.dfsBlocksExcess, prometheus.GaugeValue, nameDataMap["ExcessBlocks"].(float64))
		case "Hadoop:service=NameNode,name=FSNamesystemState":
			ch <- mustNewConstBoolMetric(e.fsOperational, prometheus.GaugeValue, nameDataMap["FSState"] == "Operational")
			ch <- prometheus.MustNewConstMetric(e.dataNodesLive, prometheus.GaugeValue, nameDataMap["NumLiveDataNodes"].(float64))
			ch <- prometheus.MustNewConstMetric(e.dataNodesDead, prometheus.GaugeValue, nameDataMap["NumDeadDataNodes"].(float64))
			ch <- prometheus.MustNewConstMetric(e.dfsFilesTotal, prometheus.GaugeValue, nameDataMap["FilesTotal"].(float64))
			ch <- prometheus.MustNewConstMetric(e.dfsCapacityBytesTotal, prometheus.GaugeValue, nameDataMap["CapacityTotal"].(float64))
			ch <- prometheus.MustNewConstMetric(e.dfsCapacityBytesUsed, prometheus.GaugeValue, nameDataMap["CapacityUsed"].(float64))
			ch <- prometheus.MustNewConstMetric(e.dfsCapacityBytesRemaining, prometheus.GaugeValue, nameDataMap["CapacityRemaining"].(float64))
		case "Hadoop:service=NameNode,name=NameNodeInfo":
			ch <- mustNewConstBoolMetric(e.safemodeOn, prometheus.GaugeValue, nameDataMap["Safemode"] != "")
			ch <- prometheus.MustNewConstMetric(e.dfsPercentUsed, prometheus.GaugeValue, nameDataMap["PercentUsed"].(float64))
			ch <- prometheus.MustNewConstMetric(e.dfsPercentRemaining, prometheus.GaugeValue, nameDataMap["PercentRemaining"].(float64))
			ch <- prometheus.MustNewConstMetric(e.dfsNonDfsBytesUsed, prometheus.GaugeValue, nameDataMap["NonDfsUsedSpace"].(float64))
			ch <- prometheus.MustNewConstMetric(e.dfsBlockPoolBytesUsed, prometheus.GaugeValue, nameDataMap["BlockPoolUsedSpace"].(float64))
			ch <- prometheus.MustNewConstMetric(e.dfsBlockPoolPercentUsed, prometheus.GaugeValue, nameDataMap["PercentBlockPoolUsed"].(float64))
		case "Hadoop:service=NameNode,name=JvmMetrics":
			ch <- prometheus.MustNewConstMetric(e.jvmLogFatal, prometheus.CounterValue, nameDataMap["LogFatal"].(float64))
			ch <- prometheus.MustNewConstMetric(e.jvmLogError, prometheus.CounterValue, nameDataMap["LogError"].(float64))
			ch <- prometheus.MustNewConstMetric(e.jvmLogWarn, prometheus.CounterValue, nameDataMap["LogWarn"].(float64))
			ch <- prometheus.MustNewConstMetric(e.jvmLogInfo, prometheus.CounterValue, nameDataMap["LogInfo"].(float64))
			ch <- prometheus.MustNewConstMetric(e.jvmMemHeapMegabytesUsed, prometheus.GaugeValue, nameDataMap["MemHeapUsedM"].(float64))
			ch <- prometheus.MustNewConstMetric(e.jvmMemHeapMegabytesCommitted, prometheus.GaugeValue, nameDataMap["MemHeapCommittedM"].(float64))
			ch <- prometheus.MustNewConstMetric(e.jvmMemNonHeapMegabytesUsed, prometheus.GaugeValue, nameDataMap["MemNonHeapUsedM"].(float64))
			ch <- prometheus.MustNewConstMetric(e.jvmMemNonHeapMegabytesCommitted, prometheus.GaugeValue, nameDataMap["MemNonHeapCommittedM"].(float64))
			ch <- prometheus.MustNewConstMetric(e.jvmThreadsNew, prometheus.GaugeValue, nameDataMap["ThreadsNew"].(float64))
			ch <- prometheus.MustNewConstMetric(e.jvmThreadsRunnable, prometheus.GaugeValue, nameDataMap["ThreadsRunnable"].(float64))
			ch <- prometheus.MustNewConstMetric(e.jvmThreadsBlocked, prometheus.GaugeValue, nameDataMap["ThreadsBlocked"].(float64))
			ch <- prometheus.MustNewConstMetric(e.jvmThreadsWaiting, prometheus.GaugeValue, nameDataMap["ThreadsWaiting"].(float64))
			ch <- prometheus.MustNewConstMetric(e.jvmThreadsTimedWaiting, prometheus.GaugeValue, nameDataMap["ThreadsTimedWaiting"].(float64))
			ch <- prometheus.MustNewConstMetric(e.jvmThreadsTerminated, prometheus.GaugeValue, nameDataMap["ThreadsTerminated"].(float64))
		}
	}
}

func main() {
	flag.Parse()

	if *showVersion {
		fmt.Fprintln(os.Stdout, version.Print("namenode_exporter"))
		os.Exit(0)
	}

	log.Infoln("Starting namenode_exporter", version.Info())
	log.Infoln("Build context", version.BuildContext())

	prometheus.MustRegister(NewExporter(*namenodeJmxURL, *namenodeJmxTimeout))

	if *pidFile != "" {
		procExporter := prometheus.NewProcessCollectorPIDFn(func() (int, error) {
			content, err := ioutil.ReadFile(*pidFile)
			if err != nil {
				return 0, fmt.Errorf("can't read pid file %q: %s", *pidFile, err)
			}
			value, err := strconv.Atoi(strings.TrimSpace(string(content)))
			if err != nil {
				return 0, fmt.Errorf("can't parse pid file %q: %s", *pidFile, err)
			}
			return value, nil
		}, namespace)
		prometheus.MustRegister(procExporter)
	}

	landingPage := []byte(`<html>
<head><title>Namenode Exporter</title></head>
<body>
<h1>Namenode Exporter</h1>
<p><a href='` + *metricsPath + `'>Metrics</a></p>
</body>
</html>
`)

	http.Handle(*metricsPath, prometheus.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write(landingPage)
	})

	log.Infoln("Starting HTTP server on", *listenAddress)
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}
