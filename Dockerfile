FROM gcr.io/distroless/base
COPY layercake /
ENTRYPOINT ["/layercake"]
