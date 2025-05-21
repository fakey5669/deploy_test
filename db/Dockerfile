FROM mariadb

RUN apt-get update && apt-get install -y nano tzdata
RUN ln -fs /usr/share/zoneinfo/Asia/Seoul /etc/localtime && dpkg-reconfigure -f noninteractive tzdata

ENV MYSQL_ROOT_PASSWORD line9876
ENV MYSQL_HOST localhost
ENV MYSQL_TCP_PORT 3306
ENV TZ=Asia/Seoul

EXPOSE 3306

ADD ./init.sql /docker-entrypoint-initdb.d

