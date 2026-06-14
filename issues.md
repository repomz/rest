вернулся к инстансу study который у нас создавался и работал отлично, но теперь при генерации возникают какие то проблемы!
> sqlc generate
> ./bin/rest generate
format internal/app/transport/httpserver/interfaces.go: 27:83: expected ')', found ',' (and 2 more errors)
package httpserver
import (
        "context"

        "github.com/google/uuid"
        "github.com/repomz/rest_generator/internal/app/domain"
)


type StudyService interface {
        GetAllStudies(ctx context.Context) ([]domain.Study, error)
        GetStudyByID(ctx context.Context, id uuid.UUID) (domain.Study, error)
        CreateStudy(ctx context.Context, item domain.StudyDB) (domain.Study, error)
        DeleteStudy(ctx context.Context, id uuid.UUID) error
        DeleteAllStudies(ctx context.Context) error
        CreateStudy(ctx context.Context, params domain.CreateStudyParams) (domain.Study, error)
        GetStudies(ctx context.Context, params domain.GetStudiesParams) ([]domain.Study, error)
        GetStudiesByDate(ctx context.Context, params domain.GetStudiesByDateParams) ([]domain.Study, error)
        GetStudiesByDateAndStudyType(ctx context.Context, params domain.GetStudiesByDateAndStudyTypeParams) ([]domain.Study, error)
        GetStudiesByDateAndSurgeon(ctx context.Context, params domain.GetStudiesByDateAndSurgeonParams) ([]domain.Study, error)
        GetStudiesByStudyType(ctx context.Context, params domain.GetStudiesByStudyTypeParams) ([]domain.Study, error)
        GetStudiesBySurgeon(ctx context.Context, params domain.GetStudiesBySurgeonParams) ([]domain.Study, error)
        GetStudiesBySurgeonAndStudyType(ctx context.Context, params domain.GetStudiesBySurgeonAndStudyTypeParams) ([]domain.Study, error)
        GetStudyByID(ctx context.Context, params domain.GetStudyByIDParams) (domain.Study, error)
        SoftDeleteAllStudies(ctx context.Context, params domain.SoftDeleteAllStudiesParams) error
        SoftDeleteStudy(ctx context.Context, params domain.SoftDeleteStudyParams) error
        GetStudiesByFilter(ctx context.Context, params domain.GetStudiesByFilterParams) (, error)
        GetStudyByPatient(ctx context.Context, params domain.GetStudyByPatientParams) (, error)
        UpdateStudyDicomLink(ctx context.Context, params domain.UpdateStudyDicomLinkParams) (, error)
}
