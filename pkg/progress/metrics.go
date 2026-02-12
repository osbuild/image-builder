package progress

import (
	"cmp"
	"slices"
	"time"

	"github.com/osbuild/images/pkg/osbuild"
)

type osbuildStageMetrics struct {
	durations []measurement
}

type measurement struct {
	Pipeline string
	Name     string
	Duration time.Duration
}

func (s *osbuildStageMetrics) Add(st *osbuild.Status) {
	if st.Duration > 1*time.Second && st.Message != "" {
		s.durations = append(s.durations, measurement{
			Pipeline: st.Pipeline,
			Name:     st.Message,
			Duration: st.Duration,
		})
	}
}

func (s *osbuildStageMetrics) String() string {
	if len(s.durations) == 0 {
		return ""
	}

	// sort measurements by duration descending
	slices.SortFunc(s.durations, func(a, b measurement) int {
		return cmp.Compare(b.Duration, a.Duration)
	})

	result := "Metrics:\n"
	for i, m := range s.durations {
		result += "\t" + m.Pipeline + ": " + m.Name + ": " + m.Duration.Truncate(time.Second).String() + "\n"
		if i >= 9 {
			break
		}
	}

	return result
}

func (s *osbuildStageMetrics) Bytes() []byte {
	return []byte(s.String())
}
