package nodeutils

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	prom "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

func StateSyncChunkMetricsExist(ctx context.Context, metricsURL string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, metricsURL, nil)
	if err != nil {
		return false, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(resp.Body)
		return false, fmt.Errorf("GET %s: %s: %s", metricsURL, resp.Status, strings.TrimSpace(string(b)))
	}

	parser := expfmt.TextParser{}
	fams, err := parser.TextToMetricFamilies(resp.Body)
	if err != nil {
		return false, err
	}

	hasReq := hasMessageType(fams["cometbft_p2p_message_send_bytes_total"], "statesync_ChunkRequest")
	hasResp := hasMessageType(fams["cometbft_p2p_message_receive_bytes_total"], "statesync_ChunkResponse")
	return hasReq || hasResp, nil
}

func hasMessageType(mf *prom.MetricFamily, want string) bool {
	if mf == nil {
		return false
	}
	for _, m := range mf.Metric {
		for _, lp := range m.Label {
			if lp.GetName() == "message_type" && lp.GetValue() == want {
				return true
			}
		}
	}
	return false
}
