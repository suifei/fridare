/* MinGW windns.h often lacks DNS Service Discovery types required by Frida 17.x */
#ifndef FRIDARE_MINGW_DNS_STUB_H
#define FRIDARE_MINGW_DNS_STUB_H

#ifndef DNS_SERVICE_CANCEL
typedef struct _DNS_SERVICE_CANCEL {
  void *reserved;
} DNS_SERVICE_CANCEL;
#endif

#ifndef DNS_REQUEST_PENDING
# define DNS_REQUEST_PENDING 9506L
#endif
#ifndef DNS_QUERY_REQUEST_VERSION2
# define DNS_QUERY_REQUEST_VERSION2 2
#endif

#ifndef FRIDARE_DNS_QUERY_RESULT_DEFINED
# define FRIDARE_DNS_QUERY_RESULT_DEFINED 1
# ifndef DNS_QUERY_RESULT
typedef struct _DNS_QUERY_RESULT {
  unsigned long Version;
  long QueryStatus; /* DNS_STATUS */
  unsigned long long QueryOptions;
  void *pQueryRecords; /* PDNS_RECORD */
  void *Reserved;
} DNS_QUERY_RESULT;
# endif
# ifndef PDNS_QUERY_RESULT
typedef DNS_QUERY_RESULT *PDNS_QUERY_RESULT;
# endif
#endif

#ifndef DNS_QUERY_COMPLETION_ROUTINE
typedef void (__stdcall *DNS_QUERY_COMPLETION_ROUTINE)(void *pQueryContext, PDNS_QUERY_RESULT pQueryResults);
#endif
#ifndef PDNS_SERVICE_BROWSE_CALLBACK
typedef void (__stdcall *PDNS_SERVICE_BROWSE_CALLBACK)(void *pQueryContext, void *pDnsRecord);
#endif

#ifndef DNS_SERVICE_BROWSE_REQUEST
typedef struct _DNS_SERVICE_BROWSE_REQUEST {
  unsigned long Version;
  unsigned long InterfaceIndex;
  const unsigned short *QueryName; /* wchar_t */
  union {
    PDNS_SERVICE_BROWSE_CALLBACK pBrowseCallback;
    DNS_QUERY_COMPLETION_ROUTINE pBrowseCallbackV2;
  };
  void *pQueryContext;
} DNS_SERVICE_BROWSE_REQUEST;
#endif

#endif /* FRIDARE_MINGW_DNS_STUB_H */
