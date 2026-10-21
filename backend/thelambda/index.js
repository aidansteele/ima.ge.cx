// Go replaces Node entirely. It binds AWS_LAMBDA_HTTP_ENDPOINT, announces
// readiness, and handles SIGTERM; no Node proxy or Lambda Web Adapter remains.
process.execve("/var/task/server", ["/var/task/server"], {
  ...process.env,
  PATH: "/var/task/bin:/usr/local/bin:/usr/bin:/bin",
  HOME: "/tmp",
});
