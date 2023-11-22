FROM kindest/node:v1.25.3
RUN clean-install tcpdump && clean-install iputils-ping && clean-install iptables && clean-install net-tools
