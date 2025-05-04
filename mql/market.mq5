//+------------------------------------------------------------------+
//| Expert Advisor to send last Buy (Ask) and Sell (Bid) as JSON      |
//+------------------------------------------------------------------+
#property copyright "Amirhossein Akhlaghpour"
#property link      "https://www.yourwebsite.com"
#property version   "1.00"

// Input parameters
input string BackendURL = "http://172.30.35.174:8080/api/prices";

//+------------------------------------------------------------------+
int OnInit()
{
   // Ensure WebRequest is allowed for the backend URL
   if(!TerminalInfoInteger(TERMINAL_DLLS_ALLOWED))
   {
      Print("DLLs are not allowed. Please enable DLLs in MT5 settings.");
      return(INIT_FAILED);
   }
   return(INIT_SUCCEEDED);
}

//+------------------------------------------------------------------+
//| Expert tick function                                             |
//+------------------------------------------------------------------+
void OnTick()
{
   // Get the current symbol
   string symbol = Symbol();
   
   // Get the last Ask (Buy) and Bid (Sell) prices
   double ask_price = SymbolInfoDouble(symbol, SYMBOL_ASK);
   double bid_price = SymbolInfoDouble(symbol, SYMBOL_BID);
   
   // Create JSON payload
   string json = StringFormat("{\"symbol\":\"%s\",\"ask\":%.5f,\"bid\":%.5f,\"timestamp\":%lld}",
                              symbol, ask_price, bid_price, TimeCurrent());
   
   // Prepare HTTP request
   string headers = "Content-Type: application/json";
   char post[], result[];
   string result_headers; // Added to store response headers
   StringToCharArray(json, post);
   
   // Send POST request to backend
   int res = WebRequest("POST", BackendURL, headers, 10000, post, result, result_headers);
   if(res == 200)
      Print("Prices sent successfully: ", json);
   else
      Print("Failed to send prices. Error code: ", res, ", JSON: ", json);
}

//+------------------------------------------------------------------+
//| Expert deinitialization function                                   |
//+------------------------------------------------------------------+
void OnDeinit(const int reason)
{
   Print("EA stopped. Reason: ", reason);
}
//+------------------------------------------------------------------+----------+