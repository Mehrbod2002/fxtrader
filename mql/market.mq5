#property copyright "Amirhossein Akhlaghpour"
#property link      "https://www.crypex.org"
#property version   "1.00"

input string BackendURL = "http://144.76.169.71:7000/api/v1/prices";

int OnInit()
{
   if(!TerminalInfoInteger(TERMINAL_DLLS_ALLOWED))
   {
      return(INIT_FAILED);
   }
   return(INIT_SUCCEEDED);
}

void OnTick()
{
   string symbol = Symbol();
   
   double ask_price = SymbolInfoDouble(symbol, SYMBOL_ASK);
   double bid_price = SymbolInfoDouble(symbol, SYMBOL_BID);
   
   string json = StringFormat("{\"symbol\":\"%s\",\"ask\":%.5f,\"bid\":%.5f,\"timestamp\":%lld}",
                              symbol, ask_price, bid_price, TimeCurrent());
   
   string headers = "Content-Type: application/json";
   char post[], result[];
   string result_headers;
   StringToCharArray(json, post);
   
   WebRequest("POST", BackendURL, headers, 10000, post, result, result_headers);
}

void OnDeinit(const int reason)
{
   Print("EA stopped. Reason: ", reason);
}