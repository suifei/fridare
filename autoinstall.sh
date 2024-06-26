#!/bin/bash
iSSH_ROOT_DIR=`cat ~/.issh/rootdir`
if [ -z "$iSSH_ROOT_DIR" ]; then
  echo "issh not found in PATH"
  echo "Please install issh from https://github.com/4ch12dy/issh"
  exit 1
fi

DEB_NAME=$(cd dist && ls -lt *-arm_*.deb | head -n 1 | awk '{print $9}' && cd ..)
INSTALL_NAME=$(echo $DEB_NAME | awk -F'_' '{print $4}')
INSTANCE_NAME=$(source $iSSH_ROOT_DIR/issh.sh && issh run "apt install gawk -y --allow-unauthenticated" && issh run "ps -e | grep 0.0.0.0:8899 | grep /usr/sbin/ | grep -v grep | awk '{n=split(\$0,a,\"/\");n2=split(a[n],b,\" \"); print b[n2-2]}'")
INSTANCE_NAME=$(echo $INSTANCE_NAME | awk '{print $NF}')
echo "DEB File: $DEB_NAME"
echo "Frida Instance: $INSTANCE_NAME"
echo "Install Name: $INSTALL_NAME"

source $iSSH_ROOT_DIR/issh.sh && issh scp dist/$DEB_NAME

if [ -n "$INSTANCE_NAME" ]; then
    source $iSSH_ROOT_DIR/issh.sh && issh run "dpkg -r re.$INSTANCE_NAME.server"  
fi
source $iSSH_ROOT_DIR/issh.sh && issh run "dpkg -i /var/root/$DEB_NAME"
source $iSSH_ROOT_DIR/issh.sh && issh run "rm -rf /var/root/$DEB_NAME"
source $iSSH_ROOT_DIR/issh.sh && issh run "ps -e|grep $INSTALL_NAME"