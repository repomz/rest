package httpserver

type HttpServer struct {
	agent_recordService AgentRecordService
	studyService        StudyService
}

func NewHttpServer(
	agent_recordService AgentRecordService,
	studyService StudyService,
) HttpServer {
	return HttpServer{
		agent_recordService: agent_recordService,
		studyService:        studyService,
	}
}
