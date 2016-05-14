FROM gcr.io/google_containers/ubuntu-slim:0.2

# specify the git user's home and name
ENV GITHOME /home/git
ENV GITUSER git

ENV DEIS_BUILDER_SERVER_SSH_HOST_IP=0.0.0.0
ENV DEIS_BUILDER_SERVER_SSH_HOST_PORT=2223

EXPOSE 2223
EXPOSE 3000

# install common packages, install & configure the SSH server, and set up directories, users & perms.
RUN apt-get update && apt-get install -y --no-install-recommends \
  bash \
  sudo \
  git \
  openssh-client \
  coreutils \
  tar \
  xz-utils \
  && rm -rf /var/lib/apt/lists/* \
  && mkdir -p /var/run/sshd && rm -rf /etc/ssh/ssh_host* \
  && mkdir /apps \
  && useradd -d $GITHOME $GITUSER \
  && mkdir -p $GITHOME/.ssh && chown git:git $GITHOME/.ssh \
  && chown -R $GITUSER:$GITUSER $GITHOME \
  && addgroup --quiet --gid 2000 slug \
  && useradd slug --uid=2000 --gid=2000 \
  && usermod -a -G slug $GITUSER

COPY . /

ENV WORKFLOW_RELEASE 2.0.0-dev

ENTRYPOINT ["boot", "server"]
