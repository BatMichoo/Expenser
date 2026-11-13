#!/bin/sh
# Issues nginx's TLS certificate, signed by a private home certificate
# authority (CA) that devices trust once.
#
# The CA lives in $CA_DIR (the ca-data volume) and is created on the first
# run. The server certificate, its key and the CA's public certificate
# (rootCA.crt, to install on phones and laptops) go to $OUT_DIR
# (nginx/certs). A new server certificate is issued only when there is none,
# it expires within $RENEW_DAYS days, it isn't signed by this CA, it no
# longer covers LAN_DOMAIN (and its subdomains) and LAN_IP, or FORCE is set. Otherwise nothing
# changes, so running this on every `docker compose up` is cheap.
set -eu

: "${LAN_DOMAIN:?LAN_DOMAIN is not set (see .env.template)}"
: "${LAN_IP:?LAN_IP is not set (see .env.template)}"

CA_DIR=${CA_DIR:-/ca}
OUT_DIR=${OUT_DIR:-/out}
CERT_NAME=${CERT_NAME:-homeserver.lan} # the file names nginx.conf expects
CA_DAYS=${CA_DAYS:-3650}
CERT_DAYS=${CERT_DAYS:-397}  # Apple refuses TLS certificates valid longer
RENEW_DAYS=${RENEW_DAYS:-45}  # more than autodeploy's monthly check
OUT_UID=${OUT_UID:-1000}     # owner of the files in nginx/certs, so the host
OUT_GID=${OUT_GID:-1000}     # user can read and delete them
FORCE=${FORCE:-}

ca_key=$CA_DIR/rootCA.key
ca_crt=$CA_DIR/rootCA.crt
key=$OUT_DIR/$CERT_NAME.key
crt=$OUT_DIR/$CERT_NAME.crt
org=${ORG_NAME:-Expenser}

# create_ca makes the home CA: it may only sign certificates (no further
# CAs below it), and lasts $CA_DAYS days.
create_ca() {
	echo "Creating the home certificate authority in $CA_DIR."
	(umask 077 && openssl req -x509 -new -newkey rsa:3072 -noenc -days "$CA_DAYS" \
		-keyout "$ca_key" -out "$ca_crt" -subj "/O=$org/CN=$org home CA" \
		-addext "basicConstraints=critical,CA:TRUE,pathlen:0" \
		-addext "keyUsage=critical,keyCertSign,cRLSign" \
		-addext "subjectKeyIdentifier=hash" 2>/dev/null)
}

# reason prints why a new server certificate is needed; it fails when the
# current one is still good.
reason() {
	if [ -n "$FORCE" ]; then echo "renewal was forced"; return; fi
	if [ ! -s "$crt" ] || [ ! -s "$key" ]; then echo "there is none yet"; return; fi
	if ! openssl x509 -in "$crt" -noout -checkend $((RENEW_DAYS * 86400)) >/dev/null; then
		echo "it expires within $RENEW_DAYS days"; return
	fi
	if ! openssl verify -CAfile "$ca_crt" "$crt" >/dev/null 2>&1; then
		echo "it isn't signed by the home CA"; return
	fi
	sans=$(openssl x509 -in "$crt" -noout -ext subjectAltName | tail -n +2 | tr -d ' \n')
	case ",$sans," in
	*",DNS:$LAN_DOMAIN,"*) ;;
	*) echo "LAN_DOMAIN changed"; return ;;
	esac
	case ",$sans," in
	*",DNS:*.$LAN_DOMAIN,"*) ;;
	*) echo "it doesn't cover subdomains of LAN_DOMAIN"; return ;;
	esac
	case ",$sans," in
	*",IPAddress:$LAN_IP,"*) ;;
	*) echo "LAN_IP changed"; return ;;
	esac
	return 1
}

# issue signs a new server certificate for LAN_DOMAIN, its subdomains (the
# app is served as expenser.<domain>) and LAN_IP. The files
# are written next to the old ones and then moved into place, so nginx never
# sees half a certificate.
issue() {
	tmp=$(mktemp -d)
	trap 'rm -rf "$tmp"' EXIT
	cat >"$tmp/ext" <<EXT
basicConstraints = critical, CA:FALSE
keyUsage = critical, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = DNS:$LAN_DOMAIN, DNS:*.$LAN_DOMAIN, DNS:localhost, IP:127.0.0.1, IP:$LAN_IP
subjectKeyIdentifier = hash
authorityKeyIdentifier = keyid
EXT
	(umask 077 && openssl req -new -newkey rsa:2048 -noenc -keyout "$tmp/key" \
		-out "$tmp/csr" -subj "/O=$org/CN=$LAN_DOMAIN" 2>/dev/null)
	openssl x509 -req -in "$tmp/csr" -CA "$ca_crt" -CAkey "$ca_key" \
		-set_serial "0x$(openssl rand -hex 16)" -days "$CERT_DAYS" \
		-extfile "$tmp/ext" -out "$tmp/crt" 2>/dev/null
	install -m 600 -o "$OUT_UID" -g "$OUT_GID" "$tmp/key" "$key.new"
	install -m 644 -o "$OUT_UID" -g "$OUT_GID" "$tmp/crt" "$crt.new"
	mv "$key.new" "$key"
	mv "$crt.new" "$crt"
}

mkdir -p "$OUT_DIR"
if [ ! -s "$ca_key" ] || [ ! -s "$ca_crt" ]; then
	create_ca
	FORCE=1 # certificates from an earlier CA are no longer trusted
fi
install -m 644 -o "$OUT_UID" -g "$OUT_GID" "$ca_crt" "$OUT_DIR/rootCA.crt"

if why=$(reason); then
	echo "Issuing a certificate for $LAN_DOMAIN and $LAN_IP: $why."
	issue
	openssl x509 -in "$crt" -noout -enddate | sed 's/notAfter=/Valid until /'
else
	echo "Certificate for $LAN_DOMAIN and $LAN_IP is current; nothing to do."
fi
