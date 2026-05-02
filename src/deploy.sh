go build -o BUILD/linux-x64/squ1dcc .
ftr pack . -C squ1dcc
ftr up squ1dcc*.sqar JFtR/squ1dcc
rm squ1dcc*.sqar
ftr pack . -U squ1dcc
ftr up squ1dcc*.fsdl JFtR/squ1dcc
rm squ1dcc*.fsdl
ftr query JFtR/squ1dcc