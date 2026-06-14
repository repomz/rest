package httpserver

type HttpServer struct {
	studyService StudyService
}

func NewHttpServer(
	studyService StudyService,
) HttpServer {
	return HttpServer{
		studyService: studyService,
	}
}
