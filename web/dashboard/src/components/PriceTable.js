import React, { useState, useEffect } from 'react';
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
} from '@mui/material';
import axios from 'axios';

const SYMBOLS = ['BTCUSDT', 'ETHUSDT', 'BNBUSDT', 'XRPUSDT', 'ADAUSDT'];
const API_BASE_URL = window.location.hostname === 'localhost' ? 'http://localhost:8080' : `http://${window.location.hostname}:8080`;

const PriceTable = () => {
  const [prices, setPrices] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  useEffect(() => {
    const fetchPrices = async () => {
      try {
        // Fetch prices for all symbols in parallel
        const requests = SYMBOLS.map(symbol =>
          axios.get(`${API_BASE_URL}/api/v1/prices/${symbol}`)
        );
        
        const responses = await Promise.all(requests);
        const formattedPrices = responses.map(response => ({
          id: response.data.symbol.toLowerCase(),
          name: response.data.symbol,
          price: response.data.price,
          lastUpdated: new Date(response.data.timestamp).toLocaleString(),
        }));

        setPrices(formattedPrices);
        setError(null);
      } catch (error) {
        console.error('Error fetching prices from oracle:', error);
        setError('Failed to fetch cryptocurrency prices from oracle. Please ensure the oracle server is running.');
      } finally {
        setLoading(false);
      }
    };

    fetchPrices();
    const interval = setInterval(fetchPrices, 10000); // Update every 10 seconds

    return () => clearInterval(interval);
  }, []);

  if (loading) {
    return (
      <Box display="flex" justifyContent="center" my={4}>
        <CircularProgress />
      </Box>
    );
  }

  if (error) {
    return (
      <Box display="flex" justifyContent="center" my={4}>
        <Typography color="error">{error}</Typography>
      </Box>
    );
  }

  return (
    <Paper elevation={3} sx={{ p: 2 }}>
      <Typography variant="h5" gutterBottom sx={{ mb: 3 }}>
        Oracle Price Feed
      </Typography>
      <TableContainer>
        <Table>
          <TableHead>
            <TableRow>
              <TableCell>Symbol</TableCell>
              <TableCell align="right">Price (USD)</TableCell>
              <TableCell align="right">Last Updated</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {prices.map((row) => (
              <TableRow key={row.id} hover>
                <TableCell component="th" scope="row">
                  {row.name}
                </TableCell>
                <TableCell align="right">
                  ${row.price.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
                </TableCell>
                <TableCell align="right">
                  {row.lastUpdated}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </TableContainer>
    </Paper>
  );
};

export default PriceTable; 