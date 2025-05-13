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
   ulong ticket;
};

PoolTrade pool[];
int pool_size = 0;
string tcp_host = "45.82.64.55";
int tcp_port = 7003;
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
   if (tcp_socket == INVALID_HANDLE || !SocketIsConnected(tcp_socket))
   {
      Print("Connection lost or invalid socket, attempting to reconnect...");
      if (tcp_socket != INVALID_HANDLE)
         SocketClose(tcp_socket);
      tcp_socket = SocketCreate(SOCKET_DEFAULT);
      if (tcp_socket == INVALID_HANDLE)
      {
         Print("Failed to create TCP socket. Error code: ", GetLastError());
         return;
      }
      if (!SocketConnect(tcp_socket, tcp_host, tcp_port, 5000))
      {
         Print("Failed to connect to TCP server at ", tcp_host, ":", tcp_port, ". Error code: ", GetLastError());
         SocketClose(tcp_socket);
         tcp_socket = INVALID_HANDLE;
         return;
      }
      Print("Successfully reconnected to ", tcp_host, ":", tcp_port);
   }

   string json_data = SocketReceive(tcp_socket);
   if (json_data != "")
   {
      ProcessTradeRequest(json_data);
   }
   else if (GetLastError() != 0)
   {
      Print("Socket receive error: ", GetLastError(), ", will attempt reconnect on next tick");
      SocketClose(tcp_socket);
      tcp_socket = INVALID_HANDLE;
   }

   for (int i = 0; i < pool_size; i++)
   {
      if (pool[i].trade_id == "" || pool[i].ticket == 0)
         continue;
      if (pool[i].order_type == "BUY_STOP" || pool[i].order_type == "SELL_STOP")
      {
         double market_price = SymbolInfoDouble(pool[i].symbol, SYMBOL_BID);
         bool triggered = (pool[i].order_type == "BUY_STOP" && market_price >= pool[i].entry_price) ||
                          (pool[i].order_type == "SELL_STOP" && market_price <= pool[i].entry_price);
         if (triggered)
         {
            ExecuteMarketTrade(pool[i]);
            if (pool[i].trade_id != "")
            {
               pool[i].trade_id = "";
            }
         }
      }
   }
}

string SocketReceive(int socket)
{
   string result = "";
   if (SocketIsConnected(socket))
   {
      uint len = SocketIsReadable(socket);
      if (len > 0)
      {
         uchar data[];
         ArrayResize(data, len);
         int bytes_read = SocketRead(socket, data, len, 10000);
         if (bytes_read > 0)
         {
            result = CharArrayToString(data, 0, bytes_read, CP_UTF8);
         }
      }
   }
   return result;
}

bool SocketSend(int socket, string data)
{
   uchar data_array[];
   StringToCharArray(data, data_array, 0, StringLen(data));
   return SocketSend(socket, data_array, StringLen(data)) != -1;
}

void ExecuteMarketTrade(PoolTrade &t)
{
   double price = t.order_type == "MARKET" ? SymbolInfoDouble(t.symbol, t.trade_type == "BUY" ? SYMBOL_ASK : SYMBOL_BID) : t.entry_price;
   string comment = "Trade_" + t.trade_id;
   bool success = false;
   ulong ticket = 0;

   trade.SetExpertMagicNumber(StringToInteger(t.trade_id)); // Use trade, not t
   if (t.trade_type == "BUY")
   {
      success = trade.Buy(t.volume, t.symbol, price, t.stop_loss, t.take_profit, comment); // Use trade, not t
      ticket = trade.ResultOrder();
   }
   else
   {
      success = trade.Sell(t.volume, t.symbol, price, t.stop_loss, t.take_profit, comment); // Use trade, not t
      ticket = trade.ResultOrder();
   }

   string status = success ? "EXECUTED" : "FAILED";
   SendTradeResponse(t.trade_id, t.user_id, status, "", ticket, success ? "" : trade.ResultComment());

   if (success)
   {
      t.trade_id = "";
      Print("Executed trade: ", t.trade_id, " Symbol: ", t.symbol, " Type: ", t.trade_type, " at price: ", price);
   }
   else
   {
      Print("Failed to execute trade: ", t.trade_id, ". Error: ", trade.ResultComment());
   }
}

void ProcessTradeRequest(string json_data)
{
   JSON::Object *json = new JSON::Object(json_data);
   if (!json.hasValue("type") || !json.hasValue("trade_id") || !json.hasValue("timestamp"))
   {
      Print("Invalid JSON or missing type/trade_id/timestamp: ", json_data);
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
         SendTradeResponse(json.getString("trade_id"), json.getString("user_id"), "FAILED", "", 0, "Invalid symbol");
         delete json;
         return;
      }

      double volume = json.getNumber("volume");
      double min_volume = SymbolInfoDouble(symbol, SYMBOL_VOLUME_MIN);
      double max_volume = SymbolInfoDouble(symbol, SYMBOL_VOLUME_MAX);
      if (volume < min_volume || volume > max_volume)
      {
         Print("Invalid volume: ", volume, " for symbol: ", symbol);
         SendTradeResponse(json.getString("trade_id"), json.getString("user_id"), "FAILED", "", 0, "Invalid volume");
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
      new_trade.volume = volume;
      new_trade.entry_price = json.getNumber("entry_price");
      new_trade.stop_loss = json.getNumber("stop_loss");
      new_trade.take_profit = json.getNumber("take_profit");
      new_trade.timestamp = (datetime)json.getNumber("timestamp");
      new_trade.expiration = json.getNumber("expiration") > 0 ? (datetime)json.getNumber("expiration") : 0;
      new_trade.ticket = 0;

      int match_index = FindMatchingTrade(new_trade);
      if (match_index >= 0)
      {
         ExecuteMatchedTrades(new_trade, pool[match_index]);
         if (pool[match_index].trade_id != "")
         {
            AddToPool(new_trade);
            SendTradeResponse(new_trade.trade_id, new_trade.user_id, "PENDING", "", 0, "");
         }
      }
      else
      {
         if (new_trade.order_type == "MARKET")
         {
            ExecuteMarketTrade(new_trade);
            if (new_trade.trade_id != "")
            {
               AddToPool(new_trade);
               SendTradeResponse(new_trade.trade_id, new_trade.user_id, "PENDING", "", 0, "");
            }
         }
         else
         {
            PlacePendingOrder(new_trade);
            if (new_trade.ticket > 0)
            {
               AddToPool(new_trade);
               SendTradeResponse(new_trade.trade_id, new_trade.user_id, "PENDING", "", new_trade.ticket, "");
            }
            else
            {
               SendTradeResponse(new_trade.trade_id, new_trade.user_id, "FAILED", "", 0, "Failed to place pending order");
            }
         }
      }
   }
   else if (msg_type == "balance_request")
   {
      string user_id = json.getString("user_id");
      double balance = AccountInfoDouble(ACCOUNT_BALANCE);
      SendBalanceResponse(user_id, balance, "");
   }
   else
   {
      Print("Unknown message type: ", msg_type);
   }

   delete json;
}

bool symbolExists(string symbol)
{
   return SymbolSelect(symbol, true);
}

int FindMatchingTrade(PoolTrade &new_trade)
{
   for (int i = 0; i < pool_size; i++)
   {
      if ((pool[i].trade_id == "" || pool[i].ticket == 0) && (pool[i].order_type == "BUY_STOP" || pool[i].order_type == "SELL_STOP"))
         continue;
      if (CanMatchTrades(new_trade, pool[i]))
      {
         return i;
      }
   }
   return -1;
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

   return false;
}

void ExecuteMatchedTrades(PoolTrade &trade1, PoolTrade &trade2)
{
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
   ulong ticket1 = 0, ticket2 = 0;

   trade.SetExpertMagicNumber(StringToInteger(trade1.trade_id));
   if (trade1.trade_type == "BUY")
   {
      success &= trade.Buy(trade1.volume, trade1.symbol, match_price, trade1.stop_loss, trade1.take_profit, comment);
      ticket1 = trade.ResultOrder();
      trade.SetExpertMagicNumber(StringToInteger(trade2.trade_id));
      success &= trade.Sell(trade2.volume, trade2.symbol, match_price, trade2.stop_loss, trade2.take_profit, comment);
      ticket2 = trade.ResultOrder();
   }
   else
   {
      success &= trade.Buy(trade2.volume, trade2.symbol, match_price, trade2.stop_loss, trade2.take_profit, comment);
      ticket2 = trade.ResultOrder();
      trade.SetExpertMagicNumber(StringToInteger(trade1.trade_id));
      success &= trade.Sell(trade1.volume, trade1.symbol, match_price, trade1.stop_loss, trade1.take_profit, comment);
      ticket1 = trade.ResultOrder();
   }

   string status = success ? "EXECUTED" : "FAILED";
   SendTradeResponse(trade1.trade_id, trade1.user_id, status, trade2.trade_id, ticket1, success ? "" : trade.ResultComment());
   SendTradeResponse(trade2.trade_id, trade2.user_id, status, trade1.trade_id, ticket2, success ? "" : trade.ResultComment());

   if (success)
   {
      for (int i = 0; i < pool_size; i++)
      {
         if (pool[i].trade_id == trade2.trade_id)
         {
            if (pool[i].ticket > 0)
            {
               trade.OrderDelete(pool[i].ticket);
            }
            pool[i].trade_id = "";
            break;
         }
      }
      Print("Matched trades: ", trade1.trade_id, " and ", trade2.trade_id, " at price: ", match_price);
   }
   else
   {
      Print("Failed to execute matched trades: ", trade1.trade_id, " and ", trade2.trade_id, ". Error: ", trade.ResultComment());
   }
}

void PlacePendingOrder(PoolTrade &t)
{
   string comment = "Pending_" + t.trade_id;
   bool success = false;
   t.ticket = 0;

   trade.SetExpertMagicNumber(StringToInteger(t.trade_id));
   if (t.order_type == "BUY_LIMIT")
   {
      success = trade.BuyLimit(t.volume, t.entry_price, t.symbol, t.stop_loss, t.take_profit, ORDER_TIME_SPECIFIED, t.expiration, comment);
   }
   else if (t.order_type == "SELL_LIMIT")
   {
      success = trade.SellLimit(t.volume, t.entry_price, t.symbol, t.stop_loss, t.take_profit, ORDER_TIME_SPECIFIED, t.expiration, comment);
   }
   else if (t.order_type == "BUY_STOP")
   {
      success = trade.BuyStop(t.volume, t.entry_price, t.symbol, t.stop_loss, t.take_profit, ORDER_TIME_SPECIFIED, t.expiration, comment);
   }
   else if (t.order_type == "SELL_STOP")
   {
      success = trade.SellStop(t.volume, t.entry_price, t.symbol, t.stop_loss, t.take_profit, ORDER_TIME_SPECIFIED, t.expiration, comment);
   }

   if (success)
   {
      t.ticket = trade.ResultOrder();
      Print("Placed pending order: ", t.trade_id, " Symbol: ", t.symbol, " Type: ", t.trade_type, " Order: ", t.order_type);
   }
   else
   {
      Print("Failed to place pending order: ", t.trade_id, ". Error: ", trade.ResultComment());
   }
}

void AddToPool(PoolTrade &t)
{
   ArrayResize(pool, pool_size + 1);
   pool[pool_size] = t;
   pool_size++;
   Print("Added trade to pool: ", t.trade_id, ", Symbol: ", t.symbol, ", Type: ", t.trade_type, ", Order: ", t.order_type);
}

bool SendTradeResponse(string trade_id, string user_id, string status, string matched_trade_id, ulong ticket, string error)
{
   JSON::Object *response_json = new JSON::Object();
   response_json.setProperty("type", "trade_response");
   response_json.setProperty("trade_id", trade_id);
   response_json.setProperty("user_id", user_id);
   response_json.setProperty("status", status);
   response_json.setProperty("matched_trade_id", matched_trade_id);
   response_json.setProperty("ticket", (double)ticket);
   response_json.setProperty("timestamp", (double)TimeCurrent());
   if (error != "")
   {
      response_json.setProperty("error", error);
   }

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