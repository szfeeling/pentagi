package graph

import (
	"context"
	"encoding/json"
	"fmt"

	"pentagi/pkg/graph/model"
	"pentagi/pkg/tools"
	"pentagi/pkg/wireless"
)

func (r *Resolver) wirelessFunctions(ctx context.Context, uid int64, input *model.WirelessBindingInput) (*tools.Functions, error) {
	if input == nil {
		return nil, nil
	}
	if _, _, err := validatePermission(ctx, "wireless.edit"); err != nil {
		return nil, err
	}
	if r.Config == nil || r.Config.Wireless == nil {
		return nil, fmt.Errorf("wireless service unavailable")
	}
	binding := wireless.Binding{ContextID: input.ContextID, Epoch: int64(input.Epoch)}
	if input.EndpointID != nil {
		binding.EndpointID = *input.EndpointID
	}
	if input.EndpointRevision != nil {
		binding.EndpointRevision = int64(*input.EndpointRevision)
	}
	if _, err := r.Config.Wireless.Check(ctx, uid, binding); err != nil {
		return nil, err
	}
	return &tools.Functions{Wireless: &binding}, nil
}

func wirelessJSON(value any) (map[string]interface{}, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	err = json.Unmarshal(raw, &result)
	return result, err
}

func (r *Resolver) wirelessSnapshot(ctx context.Context, uid int64) (map[string]interface{}, error) {
	if r.Config == nil || r.Config.Wireless == nil {
		return nil, fmt.Errorf("wireless service unavailable")
	}
	state, err := r.Config.Wireless.Snapshot(ctx, uid)
	if err != nil {
		return nil, err
	}
	return wirelessJSON(state)
}
