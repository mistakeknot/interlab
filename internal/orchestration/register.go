package orchestration

import "github.com/mark3labs/mcp-go/server"

func RegisterAll(s *server.MCPServer) {
	s.AddTool(PlanCampaignsTool, HandlePlanCampaigns)
	s.AddTool(DispatchCampaignsTool, HandleDispatchCampaigns)
	s.AddTool(StatusCampaignsTool, HandleStatusCampaigns)
	s.AddTool(SynthesizeCampaignsTool, HandleSynthesizeCampaigns)
}
