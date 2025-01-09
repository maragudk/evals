FROM --platform=${BUILDPLATFORM} golang AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . ./

ARG TARGETARCH
RUN GOOS=linux GOARCH=${TARGETARCH} CGO_ENABLED=1 go build -buildvcs=false -ldflags="-s -w" -o /bin/evals .



FROM scratch AS runner

COPY --from=builder /bin/evals .

ENTRYPOINT [ "./evals" ]

CMD [ "-i" , "/evals.txt" ]
