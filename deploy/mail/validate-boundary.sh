#!/bin/sh
set -eu

cfg="${1:-deploy/mail/postfix-main.cf.example}"
[ -f "$cfg" ] || { echo "missing postfix config: $cfg" >&2; exit 1; }

require() {
  pattern="$1"
  grep -Eq "$pattern" "$cfg" || { echo "required Postfix boundary rule missing: $pattern" >&2; exit 1; }
}

forbid() {
  pattern="$1"
  if grep -Eq "$pattern" "$cfg"; then
    echo "unsafe Postfix boundary rule present: $pattern" >&2
    exit 1
  fi
}

require '^[[:space:]]*smtpd_relay_restrictions[[:space:]]*=[[:space:]]*permit_mynetworks,[[:space:]]*reject_unauth_destination[[:space:]]*$'
require '^[[:space:]]*smtpd_recipient_restrictions[[:space:]]*=[[:space:]]*reject_unauth_destination[[:space:]]*$'
require '^[[:space:]]*virtual_mailbox_domains[[:space:]]*='
require '^[[:space:]]*virtual_mailbox_maps[[:space:]]*=.*recipient-map'
require '^[[:space:]]*smtpd_sasl_auth_enable[[:space:]]*=[[:space:]]*no[[:space:]]*$'
require '^[[:space:]]*disable_vrfy_command[[:space:]]*=[[:space:]]*yes[[:space:]]*$'
require '^[[:space:]]*message_size_limit[[:space:]]*=[[:space:]]*[0-9]+'
require '^[[:space:]]*smtpd_recipient_limit[[:space:]]*=[[:space:]]*[0-9]+'

# Catch-all destinations or universal trusted networks defeat recipient isolation/open-relay protection.
forbid '^[[:space:]]*mynetworks[[:space:]]*=.*0\.0\.0\.0/0'
forbid '^[[:space:]]*mynetworks[[:space:]]*=.*::/0'
forbid '^[[:space:]]*virtual_mailbox_maps[[:space:]]*=.*regexp:.*\*'

printf '%s\n' 'PASS: Postfix boundary has explicit default-deny relay and recipient validation rules'
