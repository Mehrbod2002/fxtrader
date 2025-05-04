#property copyright "Amirhossein Akhlaghpour"
#property link      "https://www.yourwebsite.com"
#property version   "1.00"

input string BackendURL = "http://172.30.35.174:8080/api/prices";

int OnInit()
{
   if(!TerminalInfoInteger(TERMINAL_DLLS_ALLOWED))
   {
      Print("DLLs are not allowed. Please enable DLLs in MT5 settings.");
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
   
   int res = WebRequest("POST", BackendURL, headers, 10000, post, result, result_headers);
   if(res == 200)
      Print("Prices sent successfully: ", json);
   else
      Print("Failed to send prices. Error code: ", res, ", JSON: ", json);
}

void OnDeinit(const int reason)
{
   Print("EA stopped. Reason: ", reason);
}