ARG MYSQL_VERSION

FROM mysql:${MYSQL_VERSION}
USER root
WORKDIR /work
ENV GOPATH=/go
ENV PATH=/go/bin:/usr/local/go/bin:"$PATH"

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
        curl \
    && rm -rf /var/lib/apt/lists/*

ARG GO_VERSION=1.16.2
RUN curl -s -f -o go.tar.gz https://dl.google.com/go/go${GO_VERSION}.linux-amd64.tar.gz \
    && tar -x -z -C /usr/local -f go.tar.gz \
    && rm go.tar.gz \
    && mkdir -p $GOPATH/src/github.com/cybozu-go

# 999 is the user ID of `mysql` user in the docker official MySQL container.
RUN mkdir -p /etc/mysql \
    && chown -R 999:999 /etc/mysql \
    && mkdir -p /etc/mysql_template \
    && chown -R 999:999 /etc/mysql_template \
    && mkdir -p /var/lib/mysql-files \
    && chown -R 999:999 /var/lib/mysql-files \
    && mkdir -p /var/log/mysql \
    && chown -R 999:999 /var/log/mysql \
    && mkdir -p /var/run/mysqld \
    && chown -R 999:999 /var/run/mysqld \
    && mkdir -p /home/mysql \
    && chown -R 999:999 /home/mysql

COPY --chown=999:999 ./testdata/template-my.cnf /etc/mysql_template/my.cnf

USER 999
ENV HOME=/home/mysql

WORKDIR $GOPATH/src/github.com/cybozu-go/moco
