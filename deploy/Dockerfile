FROM docker:28-cli

RUN apk add --no-cache git curl jq

COPY autodeploy.sh /usr/local/bin/autodeploy
RUN chmod +x /usr/local/bin/autodeploy

ENV HOME=/tmp

ENTRYPOINT ["autodeploy"]
