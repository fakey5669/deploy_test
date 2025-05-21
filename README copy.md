# K8S Control 프로젝트

이 프로젝트는 React 프론트엔드, Go 백엔드, MariaDB 데이터베이스로 구성된 웹 애플리케이션입니다.

## 프로젝트 구조

```
k8scontrol/
├── frontend/                # React 프론트엔드
│   ├── public/              # 정적 파일
│   │   ├── components/      # 리액트 컴포넌트
│   │   ├── pages/           # 페이지 컴포넌트
│   │   ├── services/        # API 서비스
│   │   ├── utils/           # 유틸리티 함수
│   │   ├── App.js           # 메인 앱 컴포넌트
│   │   └── index.js         # 진입점
│   ├── package.json         # 의존성 관리
│   └── README.md            # 프론트엔드 문서
│
├── backend/                 # Go 백엔드
│   ├── cmd/                 # 실행 파일
│   │   └── server/          # 서버 진입점
│   ├── internal/            # 내부 패키지
│   │   ├── api/             # API 핸들러
│   │   ├── config/          # 설정
│   │   ├── db/              # 데이터베이스 연결 및 모델
│   │   ├── middleware/      # 미들웨어
│   │   └── service/         # 비즈니스 로직
│   ├── pkg/                 # 공개 패키지
│   ├── go.mod               # Go 모듈 정의
│   └── README.md            # 백엔드 문서
│
├── db/                      # 데이터베이스 관련 파일
│   ├── migrations/          # 스키마 마이그레이션
│   ├── seeds/               # 초기 데이터
│   └── README.md            # 데이터베이스 문서
│
├── docker/                  # Docker 설정
│   ├── frontend/            # 프론트엔드 Docker 설정
│   ├── backend/             # 백엔드 Docker 설정
│   └── db/                  # 데이터베이스 Docker 설정
│
├── docker-compose.yml       # Docker Compose 설정
└── README.md                # 프로젝트 문서
```

## 시작하기

### 필수 조건

- Docker 및 Docker Compose
- Node.js (프론트엔드 로컬 개발용)
- Go (백엔드 로컬 개발용)

### 설치 및 실행

1. 저장소 클론:
   ```bash
   git clone <repository-url>
   cd k8scontrol
   ```

2. Docker Compose로 실행:
   ```bash
   docker-compose up
   ```

3. 브라우저에서 접속:
   ```
   http://localhost:3000
   ```

## 개발

### 프론트엔드 개발

```bash
cd frontend
npm install
npm start
```

### 백엔드 개발

```bash
cd backend
go run cmd/server/main.go
```

## API 문서

API 문서는 다음 URL에서 확인할 수 있습니다:
```
http://localhost:8080/swagger/index.html
```

## Getting started

To make it easy for you to get started with GitLab, here's a list of recommended next steps.

Already a pro? Just edit this README.md and make it your own. Want to make it easy? [Use the template at the bottom](#editing-this-readme)!

## Add your files

- [ ] [Create](https://docs.gitlab.com/ee/user/project/repository/web_editor.html#create-a-file) or [upload](https://docs.gitlab.com/ee/user/project/repository/web_editor.html#upload-a-file) files
- [ ] [Add files using the command line](https://docs.gitlab.com/ee/gitlab-basics/add-file.html#add-a-file-using-the-command-line) or push an existing Git repository with the following command:

```
cd existing_repo
git remote add origin https://gitlab.mipllab.com/lw/workflow/k8scontrol.git
git branch -M main
git push -uf origin main
```

## Integrate with your tools

- [ ] [Set up project integrations](http://gitlab.mipllab.com/lw/workflow/k8scontrol/-/settings/integrations)

## Collaborate with your team

- [ ] [Invite team members and collaborators](https://docs.gitlab.com/ee/user/project/members/)
- [ ] [Create a new merge request](https://docs.gitlab.com/ee/user/project/merge_requests/creating_merge_requests.html)
- [ ] [Automatically close issues from merge requests](https://docs.gitlab.com/ee/user/project/issues/managing_issues.html#closing-issues-automatically)
- [ ] [Enable merge request approvals](https://docs.gitlab.com/ee/user/project/merge_requests/approvals/)
- [ ] [Automatically merge when pipeline succeeds](https://docs.gitlab.com/ee/user/project/merge_requests/merge_when_pipeline_succeeds.html)

## Test and Deploy

Use the built-in continuous integration in GitLab.

- [ ] [Get started with GitLab CI/CD](https://docs.gitlab.com/ee/ci/quick_start/index.html)
- [ ] [Analyze your code for known vulnerabilities with Static Application Security Testing(SAST)](https://docs.gitlab.com/ee/user/application_security/sast/)
- [ ] [Deploy to Kubernetes, Amazon EC2, or Amazon ECS using Auto Deploy](https://docs.gitlab.com/ee/topics/autodevops/requirements.html)
- [ ] [Use pull-based deployments for improved Kubernetes management](https://docs.gitlab.com/ee/user/clusters/agent/)
- [ ] [Set up protected environments](https://docs.gitlab.com/ee/ci/environments/protected_environments.html)

***

# Editing this README

When you're ready to make this README your own, just edit this file and use the handy template below (or feel free to structure it however you want - this is just a starting point!). Thank you to [makeareadme.com](https://www.makeareadme.com/) for this template.

## Suggestions for a good README
Every project is different, so consider which of these sections apply to yours. The sections used in the template are suggestions for most open source projects. Also keep in mind that while a README can be too long and detailed, too long is better than too short. If you think your README is too long, consider utilizing another form of documentation rather than cutting out information.

## Name
Choose a self-explaining name for your project.

## Description
Let people know what your project can do specifically. Provide context and add a link to any reference visitors might be unfamiliar with. A list of Features or a Background subsection can also be added here. If there are alternatives to your project, this is a good place to list differentiating factors.

## Badges
On some READMEs, you may see small images that convey metadata, such as whether or not all the tests are passing for the project. You can use Shields to add some to your README. Many services also have instructions for adding a badge.

## Visuals
Depending on what you are making, it can be a good idea to include screenshots or even a video (you'll frequently see GIFs rather than actual videos). Tools like ttygif can help, but check out Asciinema for a more sophisticated method.

## Installation
Within a particular ecosystem, there may be a common way of installing things, such as using Yarn, NuGet, or Homebrew. However, consider the possibility that whoever is reading your README is a novice and would like more guidance. Listing specific steps helps remove ambiguity and gets people to using your project as quickly as possible. If it only runs in a specific context like a particular programming language version or operating system or has dependencies that have to be installed manually, also add a Requirements subsection.

## Usage
Use examples liberally, and show the expected output if you can. It's helpful to have inline the smallest example of usage that you can demonstrate, while providing links to more sophisticated examples if they are too long to reasonably include in the README.

## Support
Tell people where they can go to for help. It can be any combination of an issue tracker, a chat room, an email address, etc.

## Roadmap
If you have ideas for releases in the future, it is a good idea to list them in the README.

## Contributing
State if you are open to contributions and what your requirements are for accepting them.

For people who want to make changes to your project, it's helpful to have some documentation on how to get started. Perhaps there is a script that they should run or some environment variables that they need to set. Make these steps explicit. These instructions could also be useful to your future self.

You can also document commands to lint the code or run tests. These steps help to ensure high code quality and reduce the likelihood that the changes inadvertently break something. Having instructions for running tests is especially helpful if it requires external setup, such as starting a Selenium server for testing in a browser.

## Authors and acknowledgment
Show your appreciation to those who have contributed to the project.

## License
For open source projects, say how it is licensed.

## Project status
If you have run out of energy or time for your project, put a note at the top of the README saying that development has slowed down or stopped completely. Someone may choose to fork your project or volunteer to step in as a maintainer or owner, allowing your project to keep going. You can also make an explicit request for maintainers.
