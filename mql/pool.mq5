#include <Trade\Trade.mqh>
#include <Lib\json.mqh>

CTrade trade;

struct PoolTrade
{
   string trade_id;
   string user_id;
   string symbol;
   string trade_type;
   string order_type;
   int leverage;
   double volume;
   double entry_price;
   double stop_loss;
   double take_profit;
   datetime timestamp;
   datetime expiration;
};

PoolTrade pool[];
int pool_size = 0;
string tcp_host = "127.0.0.1";
int tcp_port = 7002;
double spread_tolerance = 0.0001;
int tcp_socket = INVALID_HANDLE;

int OnInit()
{
   tcp_socket = SocketCreate(SOCKET_DEFAULT);
   if (tcp_socket == INVALID_HANDLE)
   {
      Print("Failed to create TCP socket");
      return (INIT_FAILED);
   }

   if (!SocketConnect(tcp_socket, tcp_host, tcp_port, 5000))
   {
      Print("Failed to connect to TCP server at ", tcp_host, ":", tcp_port);
      SocketClose(tcp_socket);
      return (INIT_FAILED);
   }

   Print("Trade Pool EA initialized. Connected to ", tcp_host, ":", tcp_port);
   return (INIT_SUCCEEDED);
}

void OnTick()
{
   string json_data = SocketReceive(tcp_socket);
   if (json_data != "")
   {
      ProcessTradeRequest(json_data);
   }

   for (int i = 0; i < pool_size; i++)
   {
      if (pool[i].trade_id == "")
         continue;
      if (pool[i].order_type == "BUY_STOP" || pool[i].order_type == "SELL_STOP")
      {
         double market_price = SymbolInfoDouble(pool[i].symbol, SYMBOL_BID);
         if ((pool[i].order_type == "BUY_STOP" && market_price >= pool[i].entry_price) ||
             (pool[i].order_type == "SELL_STOP" && market_price <= pool[i].entry_price))
         {
            pool[i].order_type = "MARKET";
            Print("Stop order triggered: ", pool[i].trade_id, " converted to MARKET");
         }
      }
   }

   MatchTrades();
}

string SocketReceive(int socket)
{
   string result = "";
   uchar data[];
   int len = SocketRead(socket, data, 1024, 3000);
   if (len > 0)
   {
      result = CharArrayToString(data, 0, len);
   }
   return result;
}

bool SocketSend(int socket, string data)
{
   uchar data_array[];
   StringToCharArray(data, data_array, 0, StringLen(data));
   return SocketSend(socket, data_array, StringLen(data)) != -1;
}

void ProcessTradeRequest(string json_data)
{
   JSON::Object *json = new JSON::Object(json_data);
   if (!json.hasValue("type") || !json.hasValue("trade_id"))
   {
      Print("Invalid JSON or missing type/trade_id: ", json_data);
      delete json;
      return;
   }

   string msg_type = json.getString("type");
   if (msg_type == "trade_request")
   {
      string symbol = json.getString("symbol");
      if (!symbolExists(symbol))
      {
         Print("Invalid symbol: ", symbol);
         SendTradeResponse(json.getString("trade_id"), json.getString("user_id"), "FAILED", "Invalid symbol");
         delete json;
         return;
      }

      double volume = json.getNumber("volume");
      double min_volume = SymbolInfoDouble(symbol, SYMBOL_VOLUME_MIN);
      double max_volume = SymbolInfoDouble(symbol, SYMBOL_VOLUME_MAX);
      if (volume < min_volume || volume > max_volume)
      {
         Print("Invalid volume: ", volume, " for symbol: ", symbol);
         SendTradeResponse(json.getString("trade_id"), json.getString("user_id"), "FAILED", "Invalid volume");
         delete json;
         return;
      }

      PoolTrade new_trade;
      new_trade.trade_id = json.getString("trade_id");
      new_trade.user_id = json.getString("user_id");
      new_trade.symbol = json.getString("symbol");
      new_trade.trade_type = json.getString("trade_type");
      new_trade.order_type = json.getString("order_type");
      new_trade.leverage = (int)json.getNumber("leverage");
      new_trade.volume = json.getNumber("volume");
      new_trade.entry_price = json.getNumber("entry_price");
      new_trade.stop_loss = json.getNumber("stop_loss");
      new_trade.take_profit = json.getNumber("take_profit");
      new_trade.timestamp = (datetime)json.getNumber("timestamp");
      new_trade.expiration = json.getNumber("expiration") > 0 ? (datetime)json.getNumber("expiration") : 0;

      ArrayResize(pool, pool_size + 1);
      pool[pool_size] = new_trade;
      pool_size++;
      Print("Added trade to pool: ", new_trade.trade_id, ", Symbol: ", new_trade.symbol, ", Type: ", new_trade.trade_type, ", Order: ", new_trade.order_type);

      SendTradeResponse(new_trade.trade_id, new_trade.user_id, "PENDING", "");
   }
   else if (msg_type == "balance_request")
   {
      string user_id = json.getString("user_id");
      double balance = AccountBalance();
      SendBalanceResponse(user_id, balance, "");
   }
   else
   {
      Print("Unknown message type: ", msg_type);
   }

   delete json;
}

void MatchTrades()
{
   for (int i = 0; i < pool_size; i++)
   {
      if (pool[i].trade_id == "")
         continue;
      for (int j = i + 1; j < pool_size; j++)
      {
         if (pool[j].trade_id == "")
            continue;
         if (CanMatchTrades(pool[i], pool[j]))
         {
            ExecuteMatchedTrades(i, j);
            break;
         }
      }
   }
}

bool symbolExists(string symbol)
{
   int total = SymbolsTotal(false);
   for (int i = 0; i < total; i++)
   {
      if (SymbolName(i, false) == symbol)
      {
         return true;
      }
   }
   return false;
}

bool CanMatchTrades(PoolTrade &trade1, PoolTrade &trade2)
{
   if (trade1.trade_type == trade2.trade_type)
      return false;
   if (trade1.symbol != trade2.symbol)
      return false;
   if (trade1.volume != trade2.volume)
      return false;

   if (trade1.order_type == "MARKET" || trade2.order_type == "MARKET")
   {
      return true;
   }

   if ((trade1.order_type == "LIMIT" || trade1.order_type == "BUY_LIMIT" || trade1.order_type == "SELL_LIMIT") &&
       (trade2.order_type == "LIMIT" || trade2.order_type == "BUY_LIMIT" || trade2.order_type == "SELL_LIMIT"))
   {
      return MathAbs(trade1.entry_price - trade2.entry_price) <= spread_tolerance;
   }

   double market_price = SymbolInfoDouble(trade1.symbol, SYMBOL_BID);
   if (trade1.order_type == "BUY_STOP" && market_price >= trade1.entry_price)
      return true;
   if (trade1.order_type == "SELL_STOP" && market_price <= trade1.entry_price)
      return true;
   if (trade2.order_type == "BUY_STOP" && market_price >= trade2.entry_price)
      return true;
   if (trade2.order_type == "SELL_STOP" && market_price <= trade2.entry_price)
      return true;

   return false;
}

void ExecuteMatchedTrades(int index1, int index2)
{
   PoolTrade trade1 = pool[index1];
   PoolTrade trade2 = pool[index2];
   double match_price;

   if (trade1.order_type == "MARKET" || trade2.order_type == "MARKET")
   {
      match_price = SymbolInfoDouble(trade1.symbol, SYMBOL_BID);
   }
   else
   {
      match_price = (trade1.entry_price + trade2.entry_price) / 2.0;
   }

   string comment = "PoolMatch_" + trade1.trade_id + "_" + trade2.trade_id;
   bool success = true;

   if (trade1.trade_type == "BUY")
   {
      success &= trade.Buy(trade1.volume, trade1.symbol, match_price, trade1.stop_loss, trade1.take_profit, comment);
      success &= trade.Sell(trade2.volume, trade2.symbol, match_price, trade2.stop_loss, trade2.take_profit, comment);
   }
   else
   {
      success &= trade.Buy(trade2.volume, trade2.symbol, match_price, trade2.stop_loss, trade2.take_profit, comment);
      success &= trade.Sell(trade1.volume, trade1.symbol, match_price, trade1.stop_loss, trade1.take_profit, comment);
   }

   string status = success ? "MATCHED" : "FAILED";
   SendTradeResponse(trade1.trade_id, trade1.user_id, status, trade2.trade_id);
   SendTradeResponse(trade2.trade_id, trade2.user_id, status, trade1.trade_id);

   if (success)
   {
      pool[index1].trade_id = "";
      pool[index2].trade_id = "";
      Print("Matched trades: ", trade1.trade_id, " and ", trade2.trade_id, " at price: ", match_price);
   }
   else
   {
      Print("Failed to execute matched trades: ", trade1.trade_id, " and ", trade2.trade_id);
   }
}

bool SendTradeResponse(string trade_id, string user_id, string status, string matched_trade_id)
{
   JSON::Object *response_json = new JSON::Object();
   response_json.setProperty("type", "trade_response");
   response_json.setProperty("trade_id", trade_id);
   response_json.setProperty("user_id", user_id);
   response_json.setProperty("status", status);
   response_json.setProperty("matched_trade_id", matched_trade_id);
   response_json.setProperty("timestamp", (double)TimeCurrent());

   string json_str = response_json.toString();
   bool success = SocketSend(tcp_socket, json_str);
   if (success)
   {
      Print("Sent trade response: ", json_str);
   }
   else
   {
      Print("Failed to send trade response: ", json_str);
   }

   delete response_json;
   return success;
}

bool SendBalanceResponse(string user_id, double balance, string error)
{
   JSON::Object *response_json = new JSON::Object();
   response_json.setProperty("type", "balance_response");
   response_json.setProperty("user_id", user_id);
   response_json.setProperty("balance", balance);
   if (error != "")
   {
      response_json.setProperty("error", error);
   }
   response_json.setProperty("timestamp", (double)TimeCurrent());

   string json_str = response_json.toString();
   bool success = SocketSend(tcp_socket, json_str);
   if (success)
   {
      Print("Sent balance response: ", json_str);
   }
   else
   {
      Print("Failed to send balance response: ", json_str);
   }

   delete response_json;
   return success;
}

void OnDeinit(const int reason)
{
   if (tcp_socket != INVALID_HANDLE)
   {
      SocketClose(tcp_socket);
   }
   Print("Trade Pool EA deinitialized. Reason: ", reason);
}