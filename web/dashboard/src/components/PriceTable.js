import React, { useState, useEffect, useRef, useCallback } from 'react';
import {
  Paper,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  CircularProgress,
  Box,
  Typography,
  Alert,
  styled,
  Tooltip
} from '@mui/material';
import axios from 'axios';
import { formatDistanceToNow } from 'date-fns';
import SourceDetailModal from './SourceDetailModal';

const SYMBOLS = ['BTCUSDT', 'ETHUSDT', 'BNBUSDT', 'XRPUSDT', 'ADAUSDT'];
const API_BASE_URL = process.env.REACT_APP_API_BASE_URL || (window.location.hostname === 'localhost' ? 'http://localhost:8080' : `http://${window.location.hostname}:8080`);

const formatCurrency = (num) => {
    return new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD' }).format(num);
};

const formatNumber = (num, options = {}) => {
  const defaultOptions = { maximumFractionDigits: 2 };
  return new Intl.NumberFormat('en-US', { ...defaultOptions, ...options }).format(num);
};

const PriceCell = styled(TableCell)(({ theme, change }) => ({
  transition: 'background-color 0.5s ease-out',
  backgroundColor: change === 'up'
    ? theme.palette.success.light + '30'
    : change === 'down'
    ? theme.palette.error.light + '30'
    : 'inherit',
}));

const PriceTable = () => {
  const [prices, setPrices] = useState([]);
  const [previousPrices, setPreviousPrices] = useState({});
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  const [modalOpen, setModalOpen] = useState(false);
  const [selectedSymbol, setSelectedSymbol] = useState(null);
  const [sourceDetails, setSourceDetails] = useState(null);
  const [modalLoading, setModalLoading] = useState(false);
  const [modalError, setModalError] = useState(null);

  const priceChangeRef = useRef({});

  const fetchPrices = useCallback(async () => {
    try {
      const requests = SYMBOLS.map(symbol =>
        axios.get(`${API_BASE_URL}/api/v1/prices/${symbol}`)
      );

      const responses = await Promise.allSettled(requests);
      const currentPrices = {};
      const fetchedPrices = [];

      responses.forEach((result, index) => {
        const symbol = SYMBOLS[index];
        if (result.status === 'fulfilled') {
          const data = result.value.data;
          const newPrice = {
            symbol: data.symbol,
            price: data.price,
            volume: data.volume,
            source: data.source,
            timestamp: data.timestamp,
          };
          fetchedPrices.push(newPrice);
          currentPrices[symbol] = newPrice.price;
          if (previousPrices[symbol] !== undefined && previousPrices[symbol] !== newPrice.price) {
              priceChangeRef.current[symbol] = newPrice.price > previousPrices[symbol] ? 'up' : 'down';
          } else {
              delete priceChangeRef.current[symbol];
          }
        } else {
          console.error(`Error fetching price for ${symbol}:`, result.reason);
        }
      });

      setPreviousPrices(currentPrices);
      setPrices(fetchedPrices);
      if(error) setError(null);

    } catch (error) {
      console.error('Error fetching prices:', error);
      setError('Failed to fetch cryptocurrency prices. Please ensure the oracle server is running.');
    } finally {
      setTimeout(() => {
        priceChangeRef.current = {};
        setPrices(prev => [...prev]);
      }, 1000);

      if(loading) setLoading(false);
    }
  }, [previousPrices, loading, error]);

  useEffect(() => {
    fetchPrices();
    const interval = setInterval(fetchPrices, 10000);
    return () => clearInterval(interval);
  }, [fetchPrices]);

  const handleRowClick = async (symbol) => {
    if (!symbol) return;
    setSelectedSymbol(symbol);
    setModalOpen(true);
    setModalLoading(true);
    setModalError(null);
    setSourceDetails(null);
    try {
      const response = await axios.get(`${API_BASE_URL}/api/v1/prices/${symbol}/sources`);
      setSourceDetails(response.data);
    } catch (err) {
        console.error(`Error fetching source details for ${symbol}:`, err);
        let errorMsg = 'Failed to fetch source details.';
        if(err.response && err.response.data && err.response.data.error && err.response.data.error.message){
            errorMsg = err.response.data.error.message;
        }
        setModalError(errorMsg);
    } finally {
        setModalLoading(false);
    }
  };

  const handleCloseModal = () => {
    setModalOpen(false);
    setSelectedSymbol(null);
    setSourceDetails(null);
    setModalLoading(false);
    setModalError(null);
  };

  if (loading && prices.length === 0) {
    return (
      <Box display="flex" justifyContent="center" alignItems="center" sx={{ minHeight: '200px' }}>
        <CircularProgress />
      </Box>
    );
  }

  return (
    <Paper elevation={3} sx={{ p: 2, overflow: 'hidden' }}>
      <Typography variant="h5" gutterBottom sx={{ mb: 3 }}>
        Oracle Price Feed
      </Typography>

      {error && !loading && (
         <Alert severity="error" sx={{ mb: 2 }}>{error}</Alert>
      )}

      <TableContainer sx={{ maxHeight: 600 }}>
        <Table stickyHeader size="small">
          <TableHead>
            <TableRow>
              <TableCell>Symbol</TableCell>
              <TableCell align="right">Price</TableCell>
              <TableCell align="right">Volume (Aggregated)</TableCell>
              <TableCell align="right">Last Update</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {prices.map((row) => {
              const change = priceChangeRef.current[row.symbol];
              return (
                <TableRow
                   key={row.symbol}
                   hover
                   onClick={() => handleRowClick(row.symbol)}
                   sx={{ cursor: 'pointer' }}
                 >
                  <TableCell component="th" scope="row" sx={{ fontWeight: 'medium' }}>
                    {row.symbol}
                  </TableCell>
                  <PriceCell align="right" change={change}>
                     {formatCurrency(row.price)}
                  </PriceCell>
                  <TableCell align="right">
                    {formatNumber(row.volume)}
                  </TableCell>
                  <TableCell align="right">
                     <Tooltip title={new Date(row.timestamp).toLocaleString()} placement="top">
                         <span>{formatDistanceToNow(new Date(row.timestamp), { addSuffix: true })}</span>
                     </Tooltip>
                  </TableCell>
                </TableRow>
              );
            })}
             {prices.length === 0 && !loading && (
                <TableRow>
                    <TableCell colSpan={4} align="center">
                        No price data available.
                    </TableCell>
                </TableRow>
             )}
          </TableBody>
        </Table>
      </TableContainer>

      <SourceDetailModal
        open={modalOpen}
        onClose={handleCloseModal}
        symbol={selectedSymbol}
        details={sourceDetails}
        loading={modalLoading}
        error={modalError}
      />
    </Paper>
  );
};

export default PriceTable; 