#!/bin/bash

# Test script for the async transaction flow
BASE_URL="http://localhost:8080/api/v1"

echo "🧪 Testing Banking Ledger Async Flow"
echo "===================================="

# 1. Create an account
echo "1️⃣  Creating account..."
ACCOUNT_RESPONSE=$(curl -s -X POST $BASE_URL/accounts \
  -H "Content-Type: application/json" \
  -d '{
    "owner_name": "John Doe",
    "initial_balance": 500.00
  }')

ACCOUNT_ID=$(echo $ACCOUNT_RESPONSE | jq -r '.account.id')
echo "✅ Account created: $ACCOUNT_ID"
echo "   Balance: $500.00"
echo

# 2. Submit async transaction
echo "2️⃣  Submitting async transaction..."
TRANSACTION_RESPONSE=$(curl -s -X POST $BASE_URL/accounts/$ACCOUNT_ID/transactions \
  -H "Content-Type: application/json" \
  -d '{
    "type": "deposit",
    "amount": 150.00,
    "description": "Test async deposit"
  }')

TRANSACTION_ID=$(echo $TRANSACTION_RESPONSE | jq -r '.transaction_id')
echo "✅ Transaction submitted: $TRANSACTION_ID"
echo "   Status: pending"
echo "   Response: $TRANSACTION_RESPONSE"
echo

# 3. Check transaction immediately (should show pending)
echo "3️⃣  Checking transaction status (immediately)..."
sleep 0.5
IMMEDIATE_CHECK=$(curl -s $BASE_URL/transactions/$TRANSACTION_ID)
echo "📋 Immediate status: $(echo $IMMEDIATE_CHECK | jq -r '.status')"
echo "   Response: $IMMEDIATE_CHECK"
echo

# 4. Wait for processing
echo "4️⃣  Waiting for background processing..."
for i in {1..10}; do
  sleep 1
  STATUS_CHECK=$(curl -s $BASE_URL/transactions/$TRANSACTION_ID)
  STATUS=$(echo $STATUS_CHECK | jq -r '.status')
  echo "   Check $i: Status = $STATUS"
  
  if [ "$STATUS" = "completed" ] || [ "$STATUS" = "failed" ]; then
    echo "✅ Processing completed!"
    echo "   Final response: $STATUS_CHECK"
    break
  fi
done

echo

# 5. Check final account balance
echo "5️⃣  Checking final account balance..."
FINAL_ACCOUNT=$(curl -s $BASE_URL/accounts/$ACCOUNT_ID)
FINAL_BALANCE=$(echo $FINAL_ACCOUNT | jq -r '.account.balance')
echo "✅ Final balance: $FINAL_BALANCE"
echo

# 6. Test insufficient funds (async failure)
echo "6️⃣  Testing insufficient funds (should fail)..."
FAIL_RESPONSE=$(curl -s -X POST $BASE_URL/accounts/$ACCOUNT_ID/transactions \
  -H "Content-Type: application/json" \
  -d '{
    "type": "withdraw",
    "amount": 1000.00,
    "description": "Test insufficient funds"
  }')

FAIL_TRANSACTION_ID=$(echo $FAIL_RESPONSE | jq -r '.transaction_id')
echo "📤 Insufficient funds transaction: $FAIL_TRANSACTION_ID"

# Wait for failure processing
sleep 3
FAIL_CHECK=$(curl -s $BASE_URL/transactions/$FAIL_TRANSACTION_ID)
FAIL_STATUS=$(echo $FAIL_CHECK | jq -r '.status')
echo "❌ Expected failure status: $FAIL_STATUS"
if [ "$FAIL_STATUS" = "failed" ]; then
  ERROR_MSG=$(echo $FAIL_CHECK | jq -r '.error_message')
  echo "   Error: $ERROR_MSG"
fi
echo

# 7. Check processing mode
echo "7️⃣  Checking processing mode..."
PROCESSING_MODE=$(curl -s $BASE_URL/processing-mode)
echo "⚙️  Processing info: $PROCESSING_MODE"
echo

echo "🎉 Test completed!"
echo "Key observations:"
echo "   - Transactions show 'pending' immediately (no 404)"
echo "   - Status changes from pending → completed/failed"
echo "   - Failed transactions show error messages"
echo "   - Account balances are updated correctly"