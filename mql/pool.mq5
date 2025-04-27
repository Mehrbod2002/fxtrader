#include <Trade\Trade.mqh>
#include <Lib\json.mqh>
#include <Lib\UDPSocket.mqh>

CTrade trade;

struct PoolTrade {
   string trade_id;
   string user_id;
   string symbol;
   string trade_type;
   int leverage;
   double volume;
   double entry_price;
   datetime timestamp;
};

PoolTrade pool[];
int pool_size = 0;
string udp_host = "127.0.0.1";
int udp_listen_port = 5000;
int udp_response_port = 5001;
double spread_tolerance = 0.0001;
int socket_handle = INVALID_HANDLE;

int OnInit() {
   socket_handle = UDPInit(udp_host, udp_listen_port);
   if (socket_handle == INVALID_HANDLE) {
      Print("Failed to initialize UDP socket");
      return(INIT_FAILED);
   }
   Print("Trade Pool EA initialized. Listening on ", udp_host, ":", udp_listen_port);
   return(INIT_SUCCEEDED);
}

// Main tick function
void OnTick() {
   // Check for incoming UDP packets
   string json_data = UDPReceive(socket_handle);
   if (json_data != "") {
      ProcessTradeRequest(json_data);
   }
   // Match trades in the pool
   MatchTrades();
}

// Process incoming trade request
void ProcessTradeRequest(string json_data) {
   CJAVal json;
   if (!json.Deserialize(json_data)) {
      Print("Failed to parse JSON: ", json_data);
      return;
   }

   // Expect a single trade request
   PoolTrade new_trade;
   new_trade.trade_id = json["trade_id"].ToStr();
   new_trade.user_id = json["user_id"].ToStr();
   new_trade.symbol = json["symbol"].ToStr();
   new_trade.trade_type = json["trade_type"].ToStr();
   new_trade.leverage = (int)json["leverage"].ToInt();
   new_trade.volume = json["volume"].ToDbl();
   new_trade.entry_price = json["entry_price"].ToDbl();
   new_trade.timestamp = (datetime)json["timestamp"].ToInt();

   ArrayResize(pool, pool_size + 1);
   pool[pool_size] = new_trade;
   pool_size++;
   Print("Added trade to pool: ", new_trade.trade_id, ", Symbol: ", new_trade.symbol, ", Type: ", new_trade.trade_type);

   SendTradeResponse(new_trade.trade_id, new_trade.user_id, "PENDING", "");
}

void MatchTrades() {
   for (int i = 0; i < pool_size; i++) {
      if (pool[i].trade_id == "") continue;
      for (int j = i + 1; j < pool_size; j++) {
         if (pool[j].trade_id == "") continue;
         if (CanMatchTrades(pool[i], pool[j])) {
            ExecuteMatchedTrades(i, j);
            break;
         }
      }
   }
}

// Check if trades can be matched
bool CanMatchTrades(PoolTrade &trade1, PoolTrade &trade2) {
   if (trade1.trade_type == trade2.trade_type) return false;
   if (trade1.symbol != trade2.symbol) return false;
   if (trade1.volume != trade2.volume) return false;
   if (MathAbs(trade1.entry_price - trade2.entry_price) > spread_tolerance) return false;
   return true;
}

// Execute matched trades (record in MT5 history)
void ExecuteMatchedTrades(int index1, int index2) {
   PoolTrade trade1 = pool[index1];
   PoolTrade trade2 = pool[index2];
   double match_price = (trade1.entry_price + trade2.entry_price) / 2.0;

   // Log matched trades in MT5 account history
   string comment = "PoolMatch_" + trade1.trade_id + "_" + trade2.trade_id;
   bool success = true;

   if (trade1.trade_type == "BUY") {
      success &= trade.Buy(trade1.volume, trade1.symbol, match_price, 0, 0, comment);
      success &= trade.Sell(trade2.volume, trade2.symbol, match_price, 0, 0, comment);
   } else {
      success &= trade.Buy(trade2.volume, trade2.symbol, match_price, 0, 0, comment);
      success &= trade.Sell(trade1.volume, trade1.symbol, match_price, 0, 0, comment);
   }

   // Send response to backend
   string status = success ? "MATCHED" : "FAILED";
   SendTradeResponse(trade1.trade_id, trade1.user_id, status, trade2.trade_id);
   SendTradeResponse(trade2.trade_id, trade2.user_id, status, trade1.trade_id);

   // Mark trades as processed
   if (success) {
      pool[index1].trade_id = "";
      pool[index2].trade_id = "";
      Print("Matched trades: ", trade1.trade_id, " and ", trade2.trade_id, " at price: ", match_price);
   } else {
      Print("Failed to record matched trades: ", trade1.trade_id, " and ", trade2.trade_id);
   }
}

// Send response to backend via UDP
void SendTradeResponse(string trade_id, string user_id, string status, string matched_trade_id) {
   CJAVal response_json;
   response_json["trade_id"] = trade_id;
   response_json["user_id"] = user_id;
   response_json["status"] = status;
   response_json["matched_trade_id"] = matched_trade_id;
   response_json["timestamp"] = (long)TimeCurrent();

   string json_str = response_json.Serialize();
   if (!UDPSend(socket_handle, udp_host, udp_response_port, json_str)) {
      Print("Failed to send UDP response: ", json_str);
   } else {
      Print("Sent response: ", json_str);
   }
}

// Clean up
void OnDeinit(const int reason) {
   if (socket_handle != INVALID_HANDLE) {
      UDPClose(socket_handle);
   }
   Print("Trade Pool EA deinitialized. Reason: ", reason);
}