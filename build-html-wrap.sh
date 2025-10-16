pandoc -s -t html5 -o mydoc.html README.md \
  --from=markdown+hard_line_breaks \
  --css ~/Projects/smtpserver/styles/wrap.css