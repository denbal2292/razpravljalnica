package control

func (cp *ControlPlane) logNodeWarn(node *NodeInfo, msg string) {
	cp.logger.Warn(
		msg,
		"node_id", node.Info.NodeId,
		"address", node.Info.Address,
	)
}

func (cp *ControlPlane) logNodeDebug(node *NodeInfo, msg string) {
	cp.logger.Debug(
		msg,
		"node_id", node.Info.NodeId,
		"address", node.Info.Address,
	)
}

func (cp *ControlPlane) logNodeInfo(node *NodeInfo, msg string) {
	cp.logger.Info(
		msg,
		"node_id", node.Info.NodeId,
		"address", node.Info.Address,
	)
}

func (cp *ControlPlane) logNodeError(node *NodeInfo, err error, msg string) {
	cp.logger.Error(
		msg,
		"error", err,
		"node_id", node.Info.NodeId,
		"address", node.Info.Address,
	)
}
