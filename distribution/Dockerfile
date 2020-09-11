FROM fedora:31

RUN mkdir /rpms
ADD ./rpms/ /rpms/

RUN dnf localinstall -y /rpms/*

EXPOSE 8086

CMD [ "/usr/libexec/image-builder/image-builder" ]
