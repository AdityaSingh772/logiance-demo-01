package main

import (
   "context"
   "fmt"

   "github.com/Shridhar2104/logilo/graphql/models"
    pb "github.com/Shridhar2104/logilo/shipment/proto/proto"
)

type queryResolver struct {
   server *Server
}

func (r *queryResolver) GetAccountByID(ctx context.Context, email string, password string) (*models.Account, error) {
   accountResp, err := r.server.accountClient.LoginAndGetAccount(ctx, email, password)
   if err != nil {
       return nil, err
   }

//    // Fetch bank account details
//    bankAccount, err := r.server.accountClient.GetBankAccount(ctx, accountResp.ID.String())
//    if err != nil {
//        bankAccount = nil // Don't return error if bank account not found
//    }

   return &models.Account{
       ID:          accountResp.ID.String(),
       Name:        accountResp.Name,
       Password:    accountResp.Password,
       Email:       accountResp.Email,
    //    BankAccount: bankAccount,
   }, nil
}

func (r *queryResolver) Accounts(ctx context.Context, pagination PaginationInput) ([]*models.Account, error) {
   res, err := r.server.accountClient.ListAccounts(ctx, uint64(pagination.Skip), uint64(pagination.Take))
   if err != nil {
       return nil, err
   }

   accounts := make([]*models.Account, len(res))
   for i, account := range res {
       accounts[i] = &models.Account{ID: account.ID.String(), Name: account.Name}
   }
   return accounts, nil
}

func (r *queryResolver) GetOrdersForAccount(ctx context.Context, accountId string, pagination *OrderPaginationInput) (*OrderConnection, error) {
   pageSize := 20
   if pagination != nil && pagination.PageSize != nil {
       pageSize = *pagination.PageSize
   }

   page := 1
   if pagination != nil && pagination.Page != nil {
       page = *pagination.Page
   }

   resp, err := r.server.shopifyClient.GetOrdersForAccount(ctx, accountId, int32(page), int32(pageSize))
   if err != nil {
       return nil, fmt.Errorf("failed to get orders: %w", err)
   }

   edges := make([]*OrderEdge, len(resp.Orders))
   for i, order := range resp.Orders {
       edges[i] = &OrderEdge{
           Node: &models.Order{
               ID:                fmt.Sprintf("%d", order.ID),
               Name:              order.Name,
               Amount:            order.TotalPrice,
               AccountId:         accountId,
               CreatedAt:         order.CreatedAt,
               Currency:          order.Currency,
               TotalPrice:       order.TotalPrice,
               SubtotalPrice:    order.SubtotalPrice,
               TotalTax:         &order.TotalTax,
               FinancialStatus:  order.FinancialStatus,
               FulfillmentStatus: order.FulfillmentStatus,
               Customer: &models.Customer{
                   Email:     order.Customer.Email,
                   FirstName: order.Customer.FirstName,
                   LastName:  order.Customer.LastName,
                   Phone:     order.Customer.Phone,
               },
           },
       }
   }

   return &OrderConnection{
       Edges: edges,
       PageInfo: &PageInfo{
           HasNextPage:     page < int(resp.TotalPages),
           HasPreviousPage: page > 1,
           TotalPages:      int(resp.TotalPages),
           CurrentPage:     page,
       },
       TotalCount: int(resp.TotalCount),
   }, nil
}

func (r *queryResolver) GetOrder(ctx context.Context, id string) (*models.Order, error) {
   order, err := r.server.shopifyClient.GetOrder(ctx, id)
   if err != nil {
       return nil, err
   }

   return &models.Order{
       ID:                string(order.ID),
       Name:              order.Name,
       Amount:            order.TotalPrice,
       AccountId:         "",
       CreatedAt:         order.CreatedAt,
       Currency:          order.Currency,
       TotalPrice:        order.TotalPrice,
       SubtotalPrice:     order.SubtotalPrice,
       TotalDiscounts:    &order.TotalDiscounts,
       TotalTax:          &order.TotalTax,
       TaxesIncluded:     order.TaxesIncluded,
       FinancialStatus:   order.FinancialStatus,
       FulfillmentStatus: order.FulfillmentStatus,
       Customer: &models.Customer{
           Email:     order.Customer.Email,
           FirstName: order.Customer.FirstName,
           LastName:  order.Customer.LastName,
           Phone:     order.Customer.Phone,
       },
   }, nil
}

func (r *queryResolver) GetWalletDetails(ctx context.Context, input GetWalletDetailsInput) (*WalletDetailsResponse, error) {
    // Call the wallet client
    resp, err := r.server.paymentClient.GetWalletDetails(ctx, input.AccountID)
    if err != nil {
        return &WalletDetailsResponse{
            WalletDetails: nil,
            Errors: []*Error{{
                Code:    "WALLET_DETAILS_ERROR",
                Message: fmt.Sprintf("Failed to get wallet details: %v", err),
            }},
        }, nil
    }

    // Map the protobuf response to our GraphQL model
    return &WalletDetailsResponse{
        WalletDetails: &WalletDetails{
            AccountID:    resp.AccountId,
            Balance:      &resp.Balance,
        },
        Errors: nil,
    }, nil
}
// Ping is a simple health check method
func (r *queryResolver) Ping(ctx context.Context) (string, error) {
    return "pong", nil
}


// GetShipmentByOrder retrieves shipment details for a specific order
func (r *queryResolver) GetShipmentByOrder(ctx context.Context, orderID string) (*ShipmentResponse, error) {
    // Create request for the shipment service
    req := &pb.OrderTrackingRequest{
        OrderId: orderID,
    }

    // Call the shipment service
    resp, err := r.server.shipmentClient.GetShipmentByOrder(ctx, req)
    if err != nil {
        errStr:= err.Error()
        return &ShipmentResponse{
            Success: false,
            Error:  &errStr,
        }, nil
    }

    // Convert the response to GraphQL model
    return &ShipmentResponse{
        Success:    resp.Success,
        TrackingID: &resp.TrackingId,
        CourierAwb: &resp.CourierAwb,
        Label:      &resp.Label,
        Error:      &resp.Error,
    }, nil
}

// GetAccountShipments retrieves all shipments for an account with pagination
func (r *queryResolver) GetAccountShipments(ctx context.Context, input AccountShipmentsInput) (*AccountShipmentsResponse, error) {
    // Create request for the shipment service
    req := &pb.AccountShipmentsRequest{
        AccountId: input.AccountID,
        Page:      int32(input.Page),
        PageSize:  int32(input.PageSize),
    }

    // Call the shipment service
    resp, err := r.server.shipmentClient.GetAccountShipments(ctx, req)
    if err != nil {
        errStr := err.Error()

        return &AccountShipmentsResponse{
            Success: false,
            Error:   &errStr,
        }, nil
    }

    // Convert shipments to GraphQL models
    var shipments []*ShipmentInfo
    if resp.Shipments != nil {
        shipments = make([]*ShipmentInfo, len(resp.Shipments))
        for i, shipment := range resp.Shipments {
            shipments[i] = &ShipmentInfo{
                OrderNumber: shipment.OrderNumber,
                TrackingID:  shipment.TrackingId,
                CourierAwb:  shipment.CourierAwb,
                Status:      shipment.Status,
                Label:       &shipment.Label,
                CreatedAt:   shipment.CreatedAt,
            }
        }
    }

    return &AccountShipmentsResponse{
        Success:   resp.Success,
        Shipments: shipments,
        Error:     &resp.Error,
    }, nil
}

func (r *queryResolver) GetBankAccount(ctx context.Context, userID string) (*BankAccount, error) {
   bankAccount, err := r.server.accountClient.GetBankAccount(ctx, userID)
   if err != nil {
       return nil, fmt.Errorf("failed to get bank account: %w", err)
   }

   return &BankAccount{
       UserID:          bankAccount.UserID,
       AccountNumber:   bankAccount.AccountNumber,
       BeneficiaryName: bankAccount.BeneficiaryName,
       IfscCode:        bankAccount.IFSCCode,
       BankName:        bankAccount.BankName,
    //    CreatedAt:       bankAccount.CreatedAt,
    //    UpdatedAt:       bankAccount.UpdatedAt,
   }, nil
}