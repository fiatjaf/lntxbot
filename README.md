## Build Instructions
* Clone repository: `git clone https://github.com/fiatjaf/lntxbot && cd lntxbot`  
* Build: `go get && go build ./... && go test ./... && make`  
* Start requirements  
start postgres: `docker run -d --name dev-postgres -e POSTGRES_PASSWORD=Pass2020! -v ${HOME}/postgres-data/:/var/lib/postgresql/data -p 5432:5432 postgres`  
create db: `psql -h localhost -U postgres -f postgres.sql`  
start redis: `docker run -d --name redis-stack-server -p 6379:6379 redis/redis-stack-server:latest`  
download and place cliche.jar to ${HOME} folder
* Set environment variables and run it: 
`SERVICE_URL="<external URL>" PORT=3003 TELEGRAM_BOT_TOKEN=$TELEGRAM_BOT_TOKEN DATABASE_URL="user=postgres password=Pass2020! sslmode=disable" REDIS_URL="redis://localhost:6379" CLICHE_DATADIR="${HOME}/.cliche" CLICHE_JAR_PATH="${HOME}/cliche.jar" PROXY_ACCOUNT="123" go run .`
