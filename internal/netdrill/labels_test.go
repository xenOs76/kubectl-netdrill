package netdrill

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPodLabels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		owner      string
		ticket     string
		wantKeys   []string
		wantOwner  string
		wantTicket string
	}{
		{
			name:     "base only",
			wantKeys: []string{LabelApp, LabelManaged},
		},
		{
			name:      "owner only",
			owner:     "alice",
			wantKeys:  []string{LabelApp, LabelManaged, LabelOwner},
			wantOwner: "alice",
		},
		{
			name:       "ticket only",
			ticket:     "INC-1",
			wantKeys:   []string{LabelApp, LabelManaged, LabelTicket},
			wantTicket: "INC-1",
		},
		{
			name:       "owner and ticket",
			owner:      "bob",
			ticket:     "PROD-99",
			wantKeys:   []string{LabelApp, LabelManaged, LabelOwner, LabelTicket},
			wantOwner:  "bob",
			wantTicket: "PROD-99",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := PodLabels(tt.owner, tt.ticket)
			assert.Equal(t, LabelAppValue, got[LabelApp])
			assert.Equal(t, LabelManagedValue, got[LabelManaged])

			for _, k := range tt.wantKeys {
				assert.Contains(t, got, k)
			}

			if tt.wantOwner != "" {
				assert.Equal(t, tt.wantOwner, got[LabelOwner])
			} else {
				assert.NotContains(t, got, LabelOwner)
			}

			if tt.wantTicket != "" {
				assert.Equal(t, tt.wantTicket, got[LabelTicket])
			} else {
				assert.NotContains(t, got, LabelTicket)
			}
		})
	}
}
