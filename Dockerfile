FROM scratch
COPY mdcompress /mdcompress
ENTRYPOINT ["/mdcompress"]
