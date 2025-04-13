// SPDX-License-Identifier: MIT
pragma solidity ^0.8.19;

import "@openzeppelin/contracts/access/Ownable.sol";
import "@openzeppelin/contracts/security/Pausable.sol";

contract ModernOracle is Ownable, Pausable {
    struct DataFeed {
        bytes32 feedId;
        uint256 value;
        uint256 timestamp;
        uint256 confidence;
        address[] sources;
        mapping(address => uint256) sourceValues;
    }

    mapping(bytes32 => DataFeed) public dataFeeds;
    uint256 public minimumSources = 3;
    uint256 public maxDeviationPercentage = 10; // 10% max deviation

    event FeedUpdated(
        bytes32 indexed feedId,
        uint256 value,
        uint256 timestamp,
        uint256 confidence
    );

    event SourceAdded(bytes32 indexed feedId, address source);
    event SourceRemoved(bytes32 indexed feedId, address source);

    constructor() {
        _transferOwnership(msg.sender);
    }

    function updateFeed(
        bytes32 feedId,
        uint256 value,
        address source
    ) external whenNotPaused {
        require(value > 0, "Value must be positive");
        
        DataFeed storage feed = dataFeeds[feedId];
        
        // Add new source if not exists
        if (feed.sourceValues[source] == 0) {
            feed.sources.push(source);
            emit SourceAdded(feedId, source);
        }
        
        feed.sourceValues[source] = value;
        
        // Calculate median and confidence
        (uint256 medianValue, uint256 confidence) = calculateMedianAndConfidence(feed);
        
        feed.value = medianValue;
        feed.confidence = confidence;
        feed.timestamp = block.timestamp;
        
        emit FeedUpdated(feedId, medianValue, block.timestamp, confidence);
    }

    function getFeedData(bytes32 feedId) external view returns (
        uint256 value,
        uint256 timestamp,
        uint256 confidence,
        uint256 sourceCount
    ) {
        DataFeed storage feed = dataFeeds[feedId];
        return (
            feed.value,
            feed.timestamp,
            feed.confidence,
            feed.sources.length
        );
    }

    function calculateMedianAndConfidence(DataFeed storage feed) internal view returns (uint256, uint256) {
        require(feed.sources.length >= minimumSources, "Insufficient sources");
        
        uint256[] memory values = new uint256[](feed.sources.length);
        for (uint i = 0; i < feed.sources.length; i++) {
            values[i] = feed.sourceValues[feed.sources[i]];
        }
        
        // Sort values
        for (uint i = 0; i < values.length; i++) {
            for (uint j = i + 1; j < values.length; j++) {
                if (values[i] > values[j]) {
                    (values[i], values[j]) = (values[j], values[i]);
                }
            }
        }
        
        // Calculate median
        uint256 median;
        if (values.length % 2 == 0) {
            uint256 mid1 = values[values.length / 2 - 1];
            uint256 mid2 = values[values.length / 2];
            median = (mid1 + mid2) / 2;
        } else {
            median = values[values.length / 2];
        }
        
        // Calculate confidence based on deviation from median
        uint256 maxDeviation = (median * maxDeviationPercentage) / 100;
        uint256 validSources = 0;
        
        for (uint i = 0; i < values.length; i++) {
            if (abs(int256(values[i]) - int256(median)) <= int256(maxDeviation)) {
                validSources++;
            }
        }
        
        uint256 confidence = (validSources * 100) / values.length;
        return (median, confidence);
    }

    function abs(int256 x) internal pure returns (int256) {
        return x >= 0 ? x : -x;
    }

    function setMinimumSources(uint256 _minimumSources) external onlyOwner {
        minimumSources = _minimumSources;
    }

    function setMaxDeviation(uint256 _maxDeviationPercentage) external onlyOwner {
        maxDeviationPercentage = _maxDeviationPercentage;
    }

    function pause() external onlyOwner {
        _pause();
    }

    function unpause() external onlyOwner {
        _unpause();
    }
} 