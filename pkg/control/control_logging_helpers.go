package control

import "log/slog"

func (cp *ControlPlane) logNodeWarn(node *NodeInfo, msg string) {
	slog.Warn(
		msg,
		"node_id", node.Info.NodeId,
		"address", node.Info.Address,
	)
}

func (cp *ControlPlane) logNodeDebug(node *NodeInfo, msg string) {
	slog.Debug(
		msg,
		"node_id", node.Info.NodeId,
		"address", node.Info.Address,
	)
}

func (cp *ControlPlane) logNodeInfo(node *NodeInfo, msg string) {
	slog.Info(
		msg,
		"node_id", node.Info.NodeId,
		"address", node.Info.Address,
	)
}

func (cp *ControlPlane) logNodeError(node *NodeInfo, err error, msg string) {
	slog.Error(
		msg,
		"error", err,
		"node_id", node.Info.NodeId,
		"address", node.Info.Address,
	)
}
