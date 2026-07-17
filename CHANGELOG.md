# Changelog

All notable changes to `rest` are generated from
[Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/).

## [v0.1.2] - 2026-07-17


### Bug fixes


- **ci:** Upgrade Go to 1.25.12 for vulnerability fix (`a7d34d0`)


### Features


- **database:** Generate working PostgreSQL and MongoDB initializers (`cdaa833`)


- **toolchain:** Install compatible sqlc automatically (`985f8f2`)


- **docker:** Add production-ready database initialization flow (`555ebe2`)

## [v0.1.1] - 2026-07-05


### Bug fixes


- **generator:** Ignore runtime and generator-only files (`60f5c58`)


### Features


- **generator:** Generate app readme and architecture docs (`e5a4929`)

## [v0.1.0] - 2026-07-03


### Bug fixes


- Make rest doctor command checks race-safe (`ec78576`)


- **mongo:** Apply safe reload to generated files (`4498acf`)


- **config:** Reject mixed sql and mongo backends (`45e02b1`)


### CI


- **runtime:** Stabilize generated app e2e readiness (`5c67ad0`)


- **runtime:** Fix hanging runtime e2e test (`c975d51`)


### Documentation


- Fix contributing.md (`b303696`)


- **config:** Clarify data source sections (`18bea6a`)


- Simplify readme and contributing guide (`25496b3`)


- **readme:** Clarify minimum go version (`5870227`)


### Features


- **release:** Add changelog and git-cliff, automate changelog and release workflow with scripts and makefile (`ece7baf`)


- **auth:** Add auth logic (and add docker compose) (`07f9bc2`)


- **mongo:** Add mongo generation logic with auth mongo endpoints (`6e5a221`)


- **sql:** Naming all system folders and files to rest_* style (`05ac1b7`)


- **mongo:** Add swager for mongo logic (`bef3f72`)


- **mongo:** Add  logic (`7afb781`)


- **mongo:** Generate layered Mongo apps with auth and Docker support (`3031518`)


- **cli:** Add  for checking the generated project readiness (`24bfd1a`)


- **update:** Refactor  logic (`3ef4336`)


- **security:** Harden generated app middleware (`05ee528`)


- **cli:** Add command  - list discovered endpoints (`58aa1b3`)


- **init:** Offer update before project setup (`3916ca4`)


- **generator:** Add optional deployment guide (`1f9b2b9`)


- **doctor:** Add generator troubleshooting hints (`238b9aa`)


- **generator:** Add readiness and mongo graceful shutdown (`7265fc8`)


- **logging:** Harden generated app logging (`86228a1`)


- **metrics:** Generate prometheus observability for sql and mongo (`0f7841b`)


- **init:** Detect existing project and sqlc config (`fa74a4f`)


- **brand:** Add palm identity and cli welcome banner (`0948961`)


- **brand:** Add final rest logo artwork (`ed710e5`)


- **generator:** Fix logo and format generated go files with goimports (`f2e949f`)


### Maintenance


- **templates:** Translate public templates to english (`c3c950d`)


- **config:** Simplify rest yaml testing and middleware options (`2232f2d`)


- **config:** Clarify project fields and validate support features (`f2df6ab`)


- **ci:** Update govulncheck (`d696529`)


### Refactoring


- Some small fix (`dccba6a`)


- **cli:** Move command layer into internal package (`4963680`)


- **update:** Remove cosign dependency (`edf87aa`)


### Style


- **brand:** Add logo and init welcome banner (`38cee85`)


### Tests


- Add e2e and yaml validation tests (`bd7cd15`)


- **mongo:** Generate handler tests for mongo apps (`0387fc7`)

