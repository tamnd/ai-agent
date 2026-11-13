// SPDX-License-Identifier: Apache-2.0

package harvest

import (
	"context"
	"strings"
)

// The files of Epoch AI's data hub. The models file holds every model of the database; the notable models are the
// rows with notability criteria.
const (
	EpochModelsURL   = "https://epoch.ai/data/all_ai_models.csv"
	EpochHardwareURL = "https://epoch.ai/data/ml_hardware.csv"
)

func harvestEpoch(ctx context.Context, r *Run, o Options) ([]Row, error) {
	b, snap, err := r.Get(ctx, EpochModelsURL, 128<<20)
	if err != nil {
		return nil, err
	}
	models, err := EpochModels(b, snap, o)
	if err != nil {
		return nil, err
	}
	b, snap, err = r.Get(ctx, EpochHardwareURL, 32<<20)
	if err != nil {
		return nil, err
	}
	hw, err := EpochHardware(b, snap, o)
	if err != nil {
		return nil, err
	}
	return append(models, hw...), nil
}

// EpochModels reads all_ai_models.csv: one model row per line, in the order of the file, which is newest first.
// The values are Epoch's estimates and keep Epoch's attribution in their src (19.2); the abstract is not kept.
func EpochModels(b []byte, snap string, o Options) ([]Row, error) {
	t, err := readTable("epoch-ai", b, "Model", "Organization", "Publication date", "Parameters", "Training compute (FLOP)", "Link")
	if err != nil {
		return nil, err
	}
	var out []Row
	for _, rec := range t.rows {
		name := clean(t.get(rec, "Model"))
		if name == "" {
			continue
		}
		released := date(t.get(rec, "Publication date"))
		if o.Since != "" && released < o.Since {
			continue
		}
		row := Row{ID: name, Name: name, Kind: "model", SameAs: map[string]string{"epoch": name}, Snapshot: snap}
		if links := strings.Fields(t.get(rec, "Link")); len(links) > 0 {
			row.URL = strings.TrimRight(links[0], ",")
		}
		setFact(&row, "released", released, "Publication date")
		setFact(&row, "organization", list(t.get(rec, "Organization")), "Organization")
		setFact(&row, "country", list(t.get(rec, "Country (of organization)")), "Country (of organization)")
		setFact(&row, "domain", list(t.get(rec, "Domain")), "Domain")
		setFact(&row, "accessibility", t.get(rec, "Model accessibility"), "Model accessibility")
		for _, f := range []struct{ path, col string }{
			{"params.total", "Parameters"},
			{"training.compute_flop", "Training compute (FLOP)"},
			{"training.dataset_size", "Training dataset size (total)"},
			{"training.hours", "Training time (hours)"},
			{"training.hardware_quantity", "Hardware quantity"},
			{"training.cost_usd_2023", "Training compute cost (2023 USD)"},
		} {
			if v, ok := number(t.get(rec, f.col)); ok {
				setFact(&row, f.path, num(v), f.col)
			}
		}
		setFact(&row, "training.hardware", list(t.get(rec, "Training hardware")), "Training hardware")
		setFact(&row, "training.compute_confidence", t.get(rec, "Confidence"), "Confidence")
		if base := clean(t.get(rec, "Base model")); base != "" {
			row.Links = map[string][]string{"base_model": {base}}
		}
		if hw := list(t.get(rec, "Training hardware")); len(hw) > 0 {
			if row.Links == nil {
				row.Links = map[string][]string{}
			}
			row.Links["hardware"] = hw
		}
		if n := t.get(rec, "Notability criteria"); n != "" {
			setSignal(&row, "notable", true)
			row.Labels = append(row.Labels, list(n)...)
		}
		if t.get(rec, "Frontier model") == "True" {
			setSignal(&row, "frontier", true)
		}
		if c, ok := number(t.get(rec, "Citations")); ok {
			setSignal(&row, "citations", num(c))
		}
		out = append(out, row)
	}
	return keep(out, o.Limit), nil
}

// EpochHardware reads ml_hardware.csv: one accelerator per line.
func EpochHardware(b []byte, snap string, o Options) ([]Row, error) {
	t, err := readTable("epoch-ai", b, "Hardware name", "Manufacturer", "Type", "Release date")
	if err != nil {
		return nil, err
	}
	var out []Row
	for _, rec := range t.rows {
		name := clean(t.get(rec, "Hardware name"))
		if name == "" {
			continue
		}
		released := date(t.get(rec, "Release date"))
		if o.Since != "" && released < o.Since {
			continue
		}
		row := Row{ID: "hardware/" + name, Name: name, Kind: "hardware", SameAs: map[string]string{"epoch": "hardware/" + name}, Snapshot: snap}
		if links := strings.Fields(t.get(rec, "Link to datasheet")); len(links) > 0 && strings.HasPrefix(links[0], "http") {
			row.URL = links[0]
		}
		setFact(&row, "vendor", t.get(rec, "Manufacturer"), "Manufacturer")
		setFact(&row, "type", t.get(rec, "Type"), "Type")
		setFact(&row, "released", released, "Release date")
		for _, f := range []struct{ path, col string }{
			{"specs.flops_fp64", "FP64 (double precision) performance (FLOP/s)"},
			{"specs.flops_fp32", "FP32 (single precision) performance (FLOP/s)"},
			{"specs.flops_tf32", "TF32 (TensorFloat-32) performance (FLOP/s)"},
			{"specs.flops_fp16_tensor", "Tensor-FP16/BF16 performance (FLOP/s)"},
			{"specs.flops_fp8", "FP8 performance (FLOP/s)"},
			{"specs.flops_fp4", "FP4 performance (FLOP/s)"},
			{"specs.ops_int8", "INT8 performance (OP/s)"},
			{"specs.memory_bytes", "Memory (bytes)"},
			{"specs.memory_bandwidth", "Memory bandwidth (byte/s)"},
			{"specs.tdp_w", "TDP (W)"},
			{"specs.process_nm", "Process size (nm)"},
			{"price_usd", "Release price (USD)"},
		} {
			if v, ok := number(t.get(rec, f.col)); ok {
				setFact(&row, f.path, num(v), f.col)
			}
		}
		out = append(out, row)
	}
	return keep(out, o.Limit), nil
}
